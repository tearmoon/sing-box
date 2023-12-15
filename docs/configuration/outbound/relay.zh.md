### 结构

```json
{
  "type": "relay",
  "tag": "relay",

  "outbounds": [
    "proxy-a",
    "proxy-b",
    "proxy-c"
  ],
  "interrupt_exist_connections": false
}
```

### 字段

#### outbounds

==必填==

用于链接的出站标签列表。

#### interrupt_exist_connections

当选定的出站发生更改时，中断现有连接。

仅入站连接受此设置影响，内部连接将始终被中断。