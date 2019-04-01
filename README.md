kubernetes external service importer
===
1. import external service to k8s endpoints
2. perform http/tcp healthcheck on endpoints

W.I.P.

# example: health check

```
kubectl create -f- <<\EOF && kubectl get endpoints example-endpoints -o yaml --watch
apiVersion: v1
kind: Endpoints
metadata:
  name: example-endpoints
  labels:
    kube-service-importer.xiaopal.github.com/importer: ""
  annotations:
    kube-service-importer.xiaopal.github.com/probes: http uri=/ rise=2 fall=2 interval=1s timeout=5s
subsets:
  - addresses:
    - ip: 103.235.46.39
    - ip: 8.8.8.8
    ports:
    - port: 80
      protocol: TCP
EOF

```

# example: import from sources

```
kubectl create -f- <<\EOF && kubectl get endpoints example-static-endpoints -o yaml --watch
apiVersion: v1
kind: Endpoints
metadata:
  name: example-static-endpoints
  labels:
    kube-service-importer.xiaopal.github.com/importer: ""
  annotations:
    kube-service-importer.xiaopal.github.com/sources:
        static ip=103.235.46.39,8.8.8.8 port=80 protocol=TCP overwrite=yes
EOF


kubectl create -f- <<\EOF && kubectl get endpoints example-nslookup-endpoints1 -o yaml --watch
apiVersion: v1
kind: Endpoints
metadata:
  name: example-nslookup-endpoints1
  labels:
    kube-service-importer.xiaopal.github.com/importer: ""
  annotations:
    kube-service-importer.xiaopal.github.com/sources:
      nslookup host=www.google.com port=80 protocol=TCP overwrite=yes
    kube-service-importer.xiaopal.github.com/probes: tcp
EOF


kubectl create -f- <<\EOF && kubectl get endpoints example-nslookup-endpoints2 -o yaml --watch
apiVersion: v1
kind: Endpoints
metadata:
  name: example-nslookup-endpoints2
  labels:
    kube-service-importer.xiaopal.github.com/importer: ""
  annotations:
    kube-service-importer.xiaopal.github.com/sources:
      nslookup srv=_xmpp-server._tcp.google.com
    kube-service-importer.xiaopal.github.com/probes: tcp
EOF

```


# dev, build, test 

```
# dep ensure -v
CGO_ENABLED=0 GOOS=linux go build -o bin/kube-service-importer -ldflags '-s -w' cmd/*.go

bin/kube-service-importer -v=2

```

