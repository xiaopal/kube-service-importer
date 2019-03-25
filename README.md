kubernetes external service importer
===
1. import external service to k8s endpoints
2. perform http/tcp healthcheck on endpoints

W.I.P.

# dev, build, test 
```
# dep ensure -v
CGO_ENABLED=0 GOOS=linux go build -o bin/kube-service-importer -ldflags '-s -w' cmd/*.go

```

