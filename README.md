# Cloud ramp

Official repo for official business.

### Local Development

1. Make sure you have go 1.24 downloaded (or some version close)
2. For testing multiple different hosts, run `sudo nano /etc/hosts`, and then add the following domains:
```
127.0.0.1   a.domain.com
127.0.0.1   b.domain.com
127.0.0.1   c.domain.com
```

> DNS checks first examine the `/etc/hosts` file before performing a lookup, meaning that these will all resolve to localhost

3. Probably more steps later, will cross that bridge when we get there
4. Run `go run cmd/cloud-ramp/main.go` and watch our awesome application in action

> The local development process is meant to only simulate one server replication. Since this is what will be hosted on the edge, if one process works, any number of them _should_ work.