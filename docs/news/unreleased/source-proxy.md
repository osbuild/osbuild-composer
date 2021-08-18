# Using a proxy with sources

You can now use a proxy server with repository sources by adding the `proxy`
field to the source.  If the proxy server requires basic authentication you can
also set the username and password by using the optional `proxy_username` and
`proxy_password` fields.

For example:

    check_gpg = true
    check_ssl = true
    id = "f32-local"
    name = "local packages for fedora32"
    type = "yum-baseurl"
    url = "http://local/repos/fedora32/projectrepo/"
    proxy = "http://proxy.proxyurl.com:8123"
    proxy_username = "user"
    proxy_password = "password"

