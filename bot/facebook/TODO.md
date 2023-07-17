# TODO:

`[+]` Protect bot's internal c.pages managment; Make it thread-safe  
`[+]` Spread single message multiple attachments into separate internal messages  
`[+]` Implement `Deathorize` Callback Handler; remove related (page|+user) access_token(s)  
`[+]` Map[`ACCESS_TOKEN`]`APP_SECRET_PROOF` to avoid CPU+MEMORY usage for the same values  
`[ ]` HTTP Client **Rate Limiter** according to [this](https://developers.facebook.com/docs/messenger-platform/reference/send-api#limits) doc.  
>> ***NOTE:*** *For pages with large audiences, we recommend a send rate of 250 requests per second.*  
>
> **IDEA:** Grab request(s) until we will get ticket for next send API and than send a single batch request ...
