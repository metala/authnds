package main

import (
	"fmt"
	"strings"

	"github.com/nmcclain/ldap"
)

type ldapAttrs []*ldap.EntryAttribute

func (attrs *ldapAttrs) addAttribute(name, value string) {
	attrs.addAttributes(name, []string{value})
}

func (attrs *ldapAttrs) addAttributes(name string, values []string) {
	*attrs = append(*attrs, &ldap.EntryAttribute{
		Name:   name,
		Values: values,
	})
}

func (h configHandler) userLdapAttributes(u *configUser) ldapAttrs {
	attrs := ldapAttrs{}

	attrs.addAttributes("objectClass", []string{"inetOrgPerson", "person", "uidObject"})
	posixAccount := u.PosixUserID > 0 && u.PosixGroupID > 0
	if posixAccount {
		attrs.addAttribute("objectClass", "posixAccount")
	}
	// General
	attrs.addAttribute("cn", u.CommonName)
	attrs.addAttribute("uid", u.CommonName)

	// Person
	if len(u.Mail) > 0 {
		attrs.addAttribute("mail", u.Mail)
	}
	hasNames := len(u.GivenName) > 0 && len(u.Surname) > 0
	fullName := fmt.Sprintf("%s %s", u.GivenName, u.Surname)
	if hasNames {
		attrs.addAttribute("givenName", u.GivenName)
		attrs.addAttribute("sn", u.Surname)
		attrs.addAttribute("fullName", fullName)
	}

	if len(u.DisplayName) > 0 {
		attrs.addAttribute("displayName", u.DisplayName)
	} else if hasNames {
		attrs.addAttribute("displayName", fullName)
	} else {
		attrs.addAttribute("displayName", u.CommonName)
	}

	if posixAccount {
		attrs.addAttribute("uidNumber", fmt.Sprintf("%d", u.PosixUserID))
		attrs.addAttribute("gidNumber", fmt.Sprintf("%d", u.PosixGroupID))

		if len(u.LoginShell) > 0 {
			attrs.addAttribute("loginShell", u.LoginShell)
		} else {
			attrs.addAttribute("loginShell", "/bin/bash")
		}

		if len(u.Homedir) > 0 {
			attrs.addAttribute("homeDirectory", u.Homedir)
		} else {
			attrs.addAttribute("homeDirectory", "/home/"+u.CommonName)
		}

		if u.Disabled {
			attrs.addAttribute("loginDisabled", "TRUE")
		} else {
			attrs.addAttribute("loginDisabled", "FALSE`")
		}

		if len(u.SSHKeys) > 0 {
			attrs.addAttributes("sshPublicKey", u.SSHKeys)
		}
	}

	if u.Disabled {
		attrs.addAttribute("accountStatus", "inactive")
	} else {
		attrs.addAttribute("accountStatus", "active")
	}

	for _, groupCn := range u.GroupNames {
		if g := h.findGroupByCN(groupCn); g != nil {
			attrs.addAttribute("memberOf", g.distingushedName(h.cfg.Backend.BaseDN))
		}
	}

	return attrs
}

func (h configHandler) groupLdapAttributes(g *configGroup) ldapAttrs {
	attrs := ldapAttrs{}

	attrs.addAttributes("objectClass", []string{"groupOfNames"})
	attrs.addAttribute("cn", g.CommonName)
	attrs.addAttribute("description", g.Description)
	attrs.addAttributes("member", h.getGroupMembers(g.CommonName))
	return attrs
}

func (h configHandler) getGroupMembers(cn string) []string {
	names := []string{}

	for _, u := range h.cfg.Users {
		if idx := findIndex(u.GroupNames, cn); idx != -1 {
			names = append(names, u.distingushedName(h.cfg.Backend.BaseDN))
		}
	}
	return names
}

func (u configUser) distingushedName(baseDN string) string {
	return fmt.Sprintf("cn=%s,ou=users,%s", u.CommonName, baseDN)
}

func (g configGroup) distingushedName(baseDN string) string {
	return fmt.Sprintf("cn=%s,ou=groups,%s", g.CommonName, baseDN)
}

func findIndex(slice []string, element string) int {
	for i, str := range slice {
		if str == element {
			return i
		}
	}
	return -1
}

func (h configHandler) findGroupByCN(cn string) *configGroup {
	for _, g := range h.cfg.Groups {
		if g.CommonName == cn {
			return &g
		}
	}
	return nil
}

func filterLdapEntriesByBaseDN(entries []*ldap.Entry, baseDN string) []*ldap.Entry {
	filtered := []*ldap.Entry{}
	lowerBaseDN := strings.ToLower(baseDN)
	for _, entry := range entries {
		if strings.HasSuffix(entry.DN, lowerBaseDN) {
			filtered = append(filtered, entry)
		}
	}

	return filtered
}
