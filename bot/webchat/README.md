# **WebChat API**

  Method | Action
--------:|-------
**GET**  | **101** Switching Protocols (_websocket_)
**POST** | Upload media file(s)

## Upload Media file example
---

**`POST`** `/${bot-uri}?filename=`my_media_file.png  
`Content-Type:` image/png  
`Content-Length:` 25711  

```blob
{blob}
```

---

**`HTTP/1.1 200 OK`**  
`Content-Type:` application/json; charset=utf-8

```json
[
  {
    "id": 59044,
    "url": "https://dev.webitel.com/any/file/59044/download?domain_id=1&expires=1652097069867&signature=2661b5b68f499a4266c8f2d34ca01a1a28b7069ca25552c9f0909a93be578e85e8cd38b24b1eb958aec454e934ca70e54ec7eee3c041f27312fc3d926442c358",
    "mime": "image/png",
    "name": "my_media_file.png",
    "size": 25711
  }
]
```