# AuthNDS is Not a Directory Service

LDAP authentication server for developers and SOHO services. It uses a single toml config file to setup users and groups. Use it to centralize account management across your Linux servers.

### Quickstart

1. `docker build . -t authnds`
2. Check `config.toml.example` and make your own config file. 
3. `docker run --rm -v "/path/to/config.toml:/app/config.toml" -p 389:10389 authnds`
4. Test with traditional LDAP tools
   - For example: `ldapsearch -LLL -H ldap://localhost:389 -D cn=user1,ou=users,dc=example,dc=com -w secret -x -bdc=example,dc=com cn=user1`

### Usage:
```
authnds: securely expose your LDAP for external auth

Usage:
  authnds [options] -c <file|s3url>
  authnds -h --help
  authnds --version

Options:
  -c, --config <file>       Config file.
  -h, --help                Show this screen.
  --version                 Show version.
```

### Configuration:
AuthNDS can be deployed as a single server using only a local configuration file.  This is great for testing, or for production.
Here's a sample config:
```toml
[backend]
  baseDN = "dc=example,dc=com"

[[users]]
  commonName = "user"
  givenName="Ldap"
  surname="User"
  mail = "ldap.user@example.com"
  posixUserID = 5001
  posixGroupID = 5501
  userPassword = "{SSHA256}+E+iFJ27Yu1ODPH1UNKUmzOmUT06dwfghQJRHHnMsO5zYWx0"  # "secret"
  groupNames = ["developers"]

[[groups]]
  name = "developers"
  unixid = 5501
```
To create the password SHA hash, you can use this bash script: 
```
(
  export SALT="random-salt"
  export PASSWORD="secret"

  echo -n '{SSHA256}'
  (echo -n "$PASSWORD$SALT" | openssl dgst -sha256 -binary; echo -n "$SALT") | base64 -w0
  echo
)
```

### Two Factor Authentication
AuthNDS can be configured to accept OTP tokens as appended to a users password. Support is added for both **TOTP tokens** (often known by it's most prominent implementation, "Google Authenticator") and **Yubikey OTP tokens**.

When using 2FA, append the 2FA code to the end of the password when authenticating. For example, if your password is "monkey" and your otp is "123456", enter "monkey123456" as your password. 

#### TOTP Configuration
To enable TOTP authentication on a user, you can use a tool [like this](https://freeotp.github.io/qrcode.html) to generate a QR code (pick 'Timeout' and optionally let it generate a random secret for you), which can be scanned and used with the [Google Authenticator](https://play.google.com/store/apps/details?id=com.google.android.apps.authenticator2&hl=en) app. To enable TOTP authentication, configure the `otpsecret` for the user with the TOTP secret.

#### App Passwords
Additionally, you can specify an array of password hashes using the `passappsha256` for app passwords. These are not OTP validated, and are hashed in the same way as a password. This allows you to generate a long random string to be used in software which requires the ability to authenticate.

However, app passwords can be used without OTP as well.

#### Yubikey Configuration
For Yubikey OTP token authentication, first [configure your Yubikey](https://www.yubico.com/products/services-software/personalization-tools/yubikey-otp/). After this, make sure to [request a `Client ID` and `Secret key` pair](https://upgrade.yubico.com/getapikey/).

Now configure the the `yubikeyclientid` and `yubikeysecret` fields in the general section in the configuration file.

To enable Yubikey OTP authentication for a user, you must specify their Yubikey ID on the users `yubikey` field. The Yubikey ID is the first 12 characters of the Yubikey OTP, as explained in the below chart.

![Yubikey OTP](https://developers.yubico.com/OTP/otp_details.png)

When a user has been configured with either one of the OTP options, the OTP authentication is required for the user. If both are configured, either one will work.

### Building:
```unix
make all
```