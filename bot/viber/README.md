
```nginx
  # Special case for some kind of media providers to download files from our storage...
  # Rewrite: https://dev.webitel.com/any/file/69040/download[/path/file.ext]?param=query&...
  location ~ ^/any/file/\d+/download(/.*)? {
      limit_except GET OPTIONS {
          deny all;
      }
      proxy_set_header Host $host;
      proxy_set_header X-Real-IP  $remote_addr;
      proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
      proxy_send_timeout 3600s;
      proxy_read_timeout 3600s;
      rewrite ^/(any/file/\d+/download)(/.*)? /api/v2/$1 break;
      proxy_pass http://127.0.0.1:10023; # storage service node
  }

  # Default routes below !
  location ~ ^/any/file/... {
    
  }
```