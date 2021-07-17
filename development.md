# Development

## Firewalld help
[Solve no route to host whe invoke host from container by add firewalld rich rule](https://forums.docker.com/t/no-route-to-host-network-request-from-container-to-host-ip-port-published-from-other-container/39063/6)
[Firewalld examples](https://www.rootusers.com/how-to-use-firewalld-rich-rules-and-zones-for-filtering-and-nat/)
```bash
firewall-cmd --permanent --zone=public --list-rich-rules
firewall-cmd --get-default-zone
```

# Add firewall exception on dev
```bash
firewall-cmd --zone=public --add-port=8081/tcp
```

# Add firewall exception on prod (not working, not need)
[link](https://www.digitalocean.com/community/tutorials/how-to-configure-the-linux-firewall-for-docker-swarm-on-centos-7)
```
firewall-cmd --zone=public --add-port=3478/tcp  --permanent
firewall-cmd --zone=public --add-port=3478/udp  --permanent
firewall-cmd --zone=public --add-port=40000-40020/udp  --permanent
firewall-cmd --zone=public --add-port=40000-40020/tcp  --permanent
firewall-cmd --zone=public --add-port=57001-57021/tcp  --permanent
firewall-cmd --zone=public --add-port=57001-57021/udp  --permanent

firewall-cmd --reload

systemctl restart docker

firewall-cmd --list-all-zones
```

# Temporarily allow firewalld ports for usage in local network (not necessary in Fedora)
```
firewall-cmd --zone=public --add-port=8081/tcp
firewall-cmd --zone=public --add-port=3478/tcp
firewall-cmd --zone=public --add-port=3478/udp
firewall-cmd --zone=public --add-port=5000-5100/udp
```

[node check updates](https://www.npmjs.com/package/npm-check-updates)

[Error:java: invalid source release: 8](https://stackoverflow.com/a/26009627)

[Reactive, Security, Session MongoDb](https://medium.com/@hantsy/build-a-reactive-application-with-spring-boot-2-0-and-angular-de0ee5837fed)

# AAA Login
```
curl -i 'http://localhost:8060/api/login' \
  -H 'accept: application/json, text/plain, */*' \
  -H 'x-xsrf-token: aa0a1b63-7b5f-480d-9487-d62a48a32899' \
  -H 'content-type: application/x-www-form-urlencoded;charset=UTF-8' \
  -H 'cookie: XSRF-TOKEN=aa0a1b63-7b5f-480d-9487-d62a48a32899' \
  --data-raw 'username=admin&password=admin'
```


```
docker exec -t videochat_postgres_1 pg_dump -U aaa -b --create --column-inserts --serializable-deferrable
```

```
http://localhost:8081/api/user/list?userId=1&userId=-1
```



# Videochat
http://localhost:8081/public/index_old.html

Push down dummy go packages
```
go list -m -json all
```

Test:
```
go test ./... -count=1
```

# Update Go modules
https://github.com/golang/go/wiki/Modules
```bash
go get -u -t ./...
```


# Firefox enable video on non-localhost
https://lists.mozilla.org/pipermail/dev-platform/2019-February/023590.html
about:config
media.devices.insecure.enabled

# Mobile firefox enable video on non-localhost
![](./.markdown/mobile-ff-1.jpg)
![](./.markdown/mobile-ff-2.jpg)

# Validate turn server installation

Then install on client machine (your PC)
```bash
dnf install coturn-utils
```

Test (Actual value for InternalUserNamE and SeCrEt see in video.yml under turn.auth.credentials key)
```bash
turnutils_uclient -v -u InternalUserNamE -w SeCrEt your.public.ip.address
```

Correct output
```
0: Total connect time is 0
0: 2 connections are completed
1: start_mclient: msz=2, tot_send_msgs=0, tot_recv_msgs=0, tot_send_bytes ~ 0, tot_recv_bytes ~ 0
2: start_mclient: msz=2, tot_send_msgs=3, tot_recv_msgs=3, tot_send_bytes ~ 300, tot_recv_bytes ~ 300
2: start_mclient: tot_send_msgs=10, tot_recv_msgs=10
2: start_mclient: tot_send_bytes ~ 1000, tot_recv_bytes ~ 1000
2: Total transmit time is 2
2: Total lost packets 0 (0.000000%), total send dropped 0 (0.000000%)
2: Average round trip delay 11.500000 ms; min = 11 ms, max = 13 ms
2: Average jitter 0.800000 ms; min = 0 ms, max = 2 ms
```

# Run one test
```bash
go test ./... -count=1 -test.v -test.timeout=20s -p 1 -run TestExtractAuth
```


# For Github CI
```
git diff --dirstat=files,0 HEAD~1 | sed 's/^[ 0-9.]\+% //g' | cut -d'/' -f1 | uniq
```

# Generate ports
```python
for x in range(5200, 5301):
    print("""
      - target: %d
        published: %d
        protocol: udp
        mode: host""" % (x, x))
```

# Fixing fibers issue
```
# npm install --global node-gyp
$ /usr/bin/node /home/nkonev/go_1_11/videochat/frontend/node_modules/fibers/build
# yum groupinstall 'Development Tools'
```
