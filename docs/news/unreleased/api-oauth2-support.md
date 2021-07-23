# Composer-api and worker-api: OAuth2 support

Using [openshift-online/ocm-sdk](https://github.com/openshift-online/ocm-sdk-go) composer-api now
supports oauth2 authentication. To this end there's 4 new config options for the Worker and Composer
API:

- EnableJWT:  Enable or disable OAuth2 authentication.
- JWTKeysURL: Location where the certs used to verify the JWT tokens are served.
- JWTKeysCA:  Path to the CA which should be used when retrieving the certs (optional).
- JWTACLFile: Path to a yaml file containing a series of pattern match rules against the claims
  contained within the JWT (optional).

## ACL claims pattern matching format

The ACLFile should contain a list of claims and their required pattern in yaml format. Note that a
claim with a specific name can only be specified once. So if for instance a required pattern for the
`email` claim is listed twice, only one will pattern will be applied.

The pattern is verified using the golang regexp package, and follows the [RE2
syntax](https://github.com/google/re2/wiki/Syntax).

Example:
```
- claim: email
  pattern: ^.*@redhat\.com$
- claim: sub
  pattern: ^f:b3f7b485-7184-43c8-8169-37bd6d1fe4aa:myuser$
- claim: account_number
  pattern: ^(1000|1001|1002)$
- claim: account_id
  pattern: ^(5000|5005)$
```
