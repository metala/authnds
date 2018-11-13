package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/GeertJohan/yubigo"
	"github.com/nmcclain/ldap"
	"github.com/pquerna/otp/totp"
)

type configHandler struct {
	cfg         *config
	yubikeyAuth *yubigo.YubiAuth
}

func newConfigHandler(cfg *config, yubikeyAuth *yubigo.YubiAuth) Backend {
	handler := configHandler{
		cfg:         cfg,
		yubikeyAuth: yubikeyAuth}
	return handler
}

//
func (h configHandler) Bind(bindDN, bindSimplePw string, conn net.Conn) (resultCode ldap.LDAPResultCode, err error) {
	bindDN = strings.ToLower(bindDN)
	baseDNSuffix := strings.ToLower("," + h.cfg.Backend.BaseDN)
	usersOuSuffix := ",ou=users"

	log.Debug(fmt.Sprintf("Bind request: bindDN: %s, BaseDN: %s, source: %s", bindDN, h.cfg.Backend.BaseDN, conn.RemoteAddr().String()))

	stats_frontend.Add("bind_reqs", 1)

	// parse the bindDN - ensure that the bindDN ends with the BaseDN
	if !strings.HasSuffix(bindDN, baseDNSuffix) {
		log.Warning(fmt.Sprintf("Bind Error: BindDN %s not our BaseDN %s", bindDN, h.cfg.Backend.BaseDN))
		return ldap.LDAPResultInvalidCredentials, nil
	}

	userName := strings.TrimSuffix(bindDN, baseDNSuffix)
	if !strings.HasSuffix(userName, usersOuSuffix) {
		log.Warning(fmt.Sprintf("Bind Error: BindDN %s is not part of ou=users,%s", bindDN, h.cfg.Backend.BaseDN))
		return ldap.LDAPResultInvalidCredentials, nil
	}
	userName = strings.TrimPrefix(userName, "cn=")
	userName = strings.TrimSuffix(userName, usersOuSuffix)

	// find the user
	user := configUser{}
	found := false
	for _, u := range h.cfg.Users {
		if u.CommonName == userName {
			found = true
			user = u
		}
	}
	if !found {
		log.Warning(fmt.Sprintf("Bind Error: User %s not found.", userName))
		return ldap.LDAPResultInvalidCredentials, nil
	}

	validotp := false
	if len(user.Yubikey) == 0 && len(user.OTPSecret) == 0 {
		validotp = true
	}

	// Test Yubikey OTP, if exists
	if !validotp && len(user.Yubikey) > 0 && h.yubikeyAuth != nil {
		if len(bindSimplePw) > 44 {
			otp := bindSimplePw[len(bindSimplePw)-44:]
			yubikeyid := otp[0:12]

			if user.Yubikey == yubikeyid {
				bindSimplePw = bindSimplePw[:len(bindSimplePw)-44]
				_, ok, _ := h.yubikeyAuth.Verify(otp)
				validotp = ok
			}
		}
	}

	// Test OTP, if exists
	if !validotp && len(user.OTPSecret) > 0 && len(bindSimplePw) > 6 {
		otp := bindSimplePw[len(bindSimplePw)-6:]
		bindSimplePw = bindSimplePw[:len(bindSimplePw)-6]

		validotp = totp.Validate(otp, user.OTPSecret)
	}

	// finally, validate user passwords
	pwHash := sha256.New()
	pwHash.Write([]byte(bindSimplePw))
	pwHashDigest := hex.EncodeToString(pwHash.Sum(nil))

	// check app passwords first
	for index, appPw := range user.PassAppSHA256 {
		if appPw != pwHashDigest {
			log.Debug(fmt.Sprintf("Attempted to bind app pw #%d - failure as %s from %s", index, bindDN, conn.RemoteAddr().String()))
		} else {
			stats_frontend.Add("bind_successes", 1)
			log.Debug("Bind success using app pw #%d as %s from %s", index, bindDN, conn.RemoteAddr().String())
			return ldap.LDAPResultSuccess, nil
		}
	}

	// Then ensure the OTP is valid before checking the user password
	if !validotp {
		log.Warning(fmt.Sprintf("Bind Error: invalid OTP token as %s from %s", bindDN, conn.RemoteAddr().String()))
		return ldap.LDAPResultInvalidCredentials, nil
	}

	if ok, err := checkPassword(user.UserPassword, bindSimplePw); !ok {
		log.Warning(fmt.Sprintf("Bind Error: invalid userPassword verification"))
		return ldap.LDAPResultInvalidCredentials, err
	}

	stats_frontend.Add("bind_successes", 1)
	log.Debug("Bind success as %s from %s", bindDN, conn.RemoteAddr().String())
	return ldap.LDAPResultSuccess, nil
}

//
func (h configHandler) Search(bindDN string, searchReq ldap.SearchRequest, conn net.Conn) (result ldap.ServerSearchResult, err error) {
	bindDN = strings.ToLower(bindDN)
	baseDN := strings.ToLower("," + h.cfg.Backend.BaseDN)
	searchBaseDN := strings.ToLower(searchReq.BaseDN)
	log.Debug("Search request as %s from %s for %s", bindDN, conn.RemoteAddr().String(), searchReq.Filter)
	stats_frontend.Add("search_reqs", 1)

	// validate the user is authenticated and has appropriate access
	if len(bindDN) < 1 {
		return ldap.ServerSearchResult{ResultCode: ldap.LDAPResultInsufficientAccessRights}, fmt.Errorf("Search Error: Anonymous BindDN not allowed %s", bindDN)
	}
	if !strings.HasSuffix(bindDN, baseDN) {
		return ldap.ServerSearchResult{ResultCode: ldap.LDAPResultInsufficientAccessRights}, fmt.Errorf("Search Error: BindDN %s not in our BaseDN %s", bindDN, h.cfg.Backend.BaseDN)
	}
	if !strings.HasSuffix(searchBaseDN, h.cfg.Backend.BaseDN) {
		searchBaseDN = fmt.Sprintf("%s,%s", searchBaseDN, h.cfg.Backend.BaseDN) // Some applications send empty baseDN (e.g. Jenkins)
		// return ldap.ServerSearchResult{ResultCode: ldap.LDAPResultInsufficientAccessRights}, fmt.Errorf("Search Error: search BaseDN %s is not in our BaseDN %s", searchBaseDN, h.cfg.Backend.BaseDN)
	}
	// return all users in the config file - the LDAP library will filter results for us
	entries := []*ldap.Entry{}
	filterEntity, err := ldap.GetFilterObjectClass(searchReq.Filter)
	if err != nil {
		return ldap.ServerSearchResult{ResultCode: ldap.LDAPResultOperationsError}, fmt.Errorf("Search Error: error parsing filter: %s", searchReq.Filter)
	}

	traverseGroups := false
	traverseUsers := false
	switch filterEntity {
	case "posixgroup", "groupofnames":
		traverseGroups = true
	case "posixaccount", "inetorgperson", "person":
		traverseUsers = true
	case "":
		traverseUsers = true
		traverseGroups = true
	}

	if !traverseUsers && !traverseGroups {
		return ldap.ServerSearchResult{ResultCode: ldap.LDAPResultOperationsError}, fmt.Errorf("Search Error: unhandled filter type: %s [%s]", filterEntity, searchReq.Filter)
	}

	if traverseUsers {
		for _, u := range h.cfg.Users {
			attrs := h.userLdapAttributes(&u)
			dn := fmt.Sprintf("cn=%s,ou=users,%s", u.CommonName, h.cfg.Backend.BaseDN)
			entries = append(entries, &ldap.Entry{DN: dn, Attributes: attrs})
		}
	}

	if traverseGroups {
		for _, g := range h.cfg.Groups {
			attrs := h.groupLdapAttributes(&g)
			dn := fmt.Sprintf("cn=%s,ou=groups,%s", g.CommonName, h.cfg.Backend.BaseDN)
			entries = append(entries, &ldap.Entry{DN: dn, Attributes: attrs})
		}
	}

	stats_frontend.Add("search_successes", 1)
	log.Debug("AP: Search OK: %s", searchReq.Filter)
	return ldap.ServerSearchResult{
		Entries:    filterLdapEntriesByBaseDN(entries, searchReq.BaseDN),
		Referrals:  []string{},
		Controls:   []ldap.Control{},
		ResultCode: ldap.LDAPResultSuccess,
	}, nil
}

//
func (h configHandler) Close(boundDn string, conn net.Conn) error {
	stats_frontend.Add("closes", 1)
	return nil
}
