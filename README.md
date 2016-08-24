# req

HTTP services client

```sh
# Setting request defaults
$ export REQ_HOST=api.tower.pro
$ export REQ_PATH=v1 # default path prefix

# GET /v1/example/movies/search?title=Pulp+Fiction
# Host: $REQ_HOST
$ req get example movies search -- title="Pulp Fiction"
{
  "title": "Pulp Fiction",
  "year": 1994
}

# PUT /v1/pkg/example
# Host: $REQ_HOST
#
# {"repository": "git@github.com:username/repo.git", "description": "Example movies database"}
$ req put pkg example -- repository=git@github.com:username/repo.git description="Example movies database"
{
  "name": "example",
  "description": "Example movies database",
  "repository": {
    "url": "git@github.com:username/repo.git",
    "branch": "master"
  }
}

# Request with hostname ($REQ_HOST must be empty)
$ req get google.com

# File upload
$ req put pkg example -- archive=@package.zip description="Example movies database"

# Dump request to console output
# Choose one from: [-v, --verbose, -d, -debug]
$ req -v get me
GET /v1/me HTTP/1.1
Host: $REQ_HOST
...

# Create etcd API client
$ alias etcdctl="req --host etcd --path v2/keys --format form"

# GET /v2/keys/foo
# Host: etcd
$ etcdctl get foo

# PUT /v2/keys/foo
# Host: etcd
$ etcdctl put foo -- value="hello" ttl=5 prevExist=true
```
