### 结构

```json
{
  "match_all": false,
  "geoip": [
    "cn"
  ],
  "ip_cidr": [
    "10.0.0.0/24"
  ],
  "ip_is_private": false,
  "rule_set": [
    "geoip-cn"
  ],
  "invert": false,
  "server": "local",
}

```

!!! note ""

     当内容只有一项时，可以忽略 JSON 数组 [] 标签

### 字段

!!! note ""

    默认规则使用以下匹配逻辑:  
    `match_all` || `ipcidr` || `geoip` || `rule_set` || `ip_is_private`

    另外，引用的规则集可视为被合并，而不是作为一个单独的规则子项。

#### match_all

匹配所有响应。

如果该字段被设置，`invert` 字段将被忽略。

#### geoip

!!! failure "已在 sing-box 1.8.0 废弃"

    GeoIP 已废弃且可能在不久的将来移除，参阅 [迁移指南](/zh/migration/#geoip)。

匹配 GeoIP。

#### ip_cidr

匹配 IP CIDR。

#### ip_is_private

!!! question "自 sing-box 1.8.0 起"

匹配非公开 IP。

#### rule_set

!!! question "自 sing-box 1.8.0 起"

匹配[规则集](/zh/configuration/route/#rule_set)。

仅匹配规则集中的 `ip_cidr` 字段。

#### invert

反选匹配结果。

#### server

目标 DNS 服务器的标签。

如果该字段被设置，将直接使用该 DNS 服务器返回的结果。
