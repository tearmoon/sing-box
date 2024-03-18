### Structure

```json
{
  "match_all": false,
  "geoip": [
    "cn"
  ],
  "ip_cidr": [
    "10.0.0.0/24"
  ],
  "rule_set": [
    "geoip-cn"
  ],
  "ip_is_private": false,
  "invert": false,
  "server": "local",
}

```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

### Fields

!!! note ""

    The rule uses the following matching logic:  
    `match_all` || `ipcidr` || `geoip` || `rule_set` || `ip_is_private`

    Additionally, included rule sets can be considered merged rather than as a single rule sub-item.

#### match_all

Match all response.

If set, `invert` will be ignored

#### geoip

!!! failure "Deprecated in sing-box 1.8.0"

    GeoIP is deprecated and may be removed in the future, check [Migration](/migration/#migrate-geoip-to-rule-sets).

Match geoip.

#### ip_cidr

Match IP CIDR.

#### ip_is_private

!!! question "Since sing-box 1.8.0"

Match non-public IP.

#### rule_set

!!! question "Since sing-box 1.8.0"

Match [Rule Set](/configuration/route/#rule_set).

Only match `ip_cidr` fields in it.

#### invert

Invert match result.

#### server

Tag of the target dns server.

If set, response will be used which server returns.
