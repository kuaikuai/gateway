
INFINIBYTE, a lightweight data gateway written in golang.

# Features
- Auto handling upstream failure while indexing, aka nonstop indexing
- Auto detect the upstream failure in search
- Multiple write mechanism, one indexing request map to multi remote elasticsearch clusters
- Support TLS/HTTPS, generate the cert files automatically
- Support run background as daemon mode(only available on linux and macOS)
- Auto merge indexing operations to single bulk operation(WIP)
- Load balancing(indexing and search request), algorithm configurable(WIP)
- A controllable query cache layer, use redis as backend
- Index throttling or buffering, via disk based indexing queue(limit by queue length or size)
- Search throttling, limit concurrent connections to upstream(WIP)
- Builtin stats API and management UI(WIP)
- Builtin floating IP, support seamless failover and rolling upgrade


# Benchmark Test

```
[root@LINUX linux64]# ./esm -s https://elastic:pass@id.domain.cn:9343 -d https://elastic:pass@id.domain.cn:8000 -x medcl2 -y medcl23 -r -w 200 --sliced_scroll_size=40 -b 5 -t=30m
medcl2
[11-12 21:05:47] [INF] [main.go:461,main] start data migration..
Scroll 20387840 / 20387840 [===================================================================================] 100.00% 1m21s
Bulk 20375408 / 20387840 [=====================================================================================]  99.94% 2m10s
[11-12 21:07:57] [INF] [main.go:492,main] data migration finished.
```


# Docker

The docker image size is only 8.7 MB.

Pull it from official docker hub
```
docker pull medcl/infini-gateway:latest
```

Customize your `proxy.yml`, place somewhere, eg: `/tmp/proxy.yml`
```
tee /tmp/proxy.yml <<-'EOF'
elasticsearch:
- name: default
  enabled: true
  endpoint: http://192.168.3.123:9200
  index_prefix: proxy-
  basic_auth:
    username: elastic
    password: changeme
EOF
```

Rock with your proxy!
```
docker run --publish 2900:2900  -v /tmp/gateway.yml:/gateway.yml medcl/infini-gateway:latest
```

License
=======
Released under the [AGPL](https://infini.sh/LICENSE).
