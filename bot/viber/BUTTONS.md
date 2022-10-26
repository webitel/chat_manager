# **BUTTON** Options

 Metadata       | Value     | Default
----------------|-----------|---------
`btn.back.color`|*HEX Color*|`#1d2733` *(dark-gray)*
`btn.font.color`|*HEX Color*|`#ffffff` *(white)*

# **API** Setup

> **NOTE**: If gateway for yours **Viber** bot account is already created, you need to GET it's current setting, merge them with desired settings, described above, then PATCH gateway with "metadata" changes.. For example:

```curl
>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

GET /api/chat/bots/322
Content-Type: application/json; charset=utf-8
X-Webitel-Access: $access_token

<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  ...
  "metadata": {
    "token": "$your_viber_bot_token"
  },
  ...
}

>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

PATCH /api/chat/bots/322
Content-Type: application/json; charset=utf-8
X-Webitel-Access: $access_token

{
  "metadata": {
    "token": "$your_viber_bot_token",
    "btn.back.color": "#4c00b0",
    "btn.font.color": "#000000"
  }
}

<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
```
