# Docker Machine Aliyun Driver

This is a plugin for [Docker Machine](https://docs.docker.com/machine/) allowing
to create Docker hosts locally on [Aliyun](http://www.aliyun.com/)


## Installation

To install this plugin manually, download the binary `docker-machine-driver-aliyun`
and  make it available by `$PATH`, for example by putting it to `/usr/local/bin/`:

```console
$ curl -L https://github.com/venusource/docker-machine-aliyun/releases/download/v1.0.0/docker-machine-driver-aliyun > /usr/local/bin/docker-machine-driver-aliyun

$ chmod +x /usr/local/bin/docker-machine-driver-aliyun
```

The latest version of `docker-machine-driver-aliyun` binary is available on
the ["Releases"](https://github.com/venusource/docker-machine-aliyun/releases) page.

## Usage
Official documentation for Docker Machine [is available here](https://docs.docker.com/machine/).

To create a aliyun virtual machine for Docker purposes 
you must to export aliyun ACCESS_KEY_ID and ACCESS_KEY_SECRET to your environment like this:

```console
$ export ECS_ACCESS_KEY_ID="your access key id"
$ export ECS_ACCESS_KEY_SECRET="your access key secret"
```
or put it to bash_profile file.
then run this command:

```console
$ docker-machine create --driver=aliyun --security-group-id=your-security-group aliyuntest
```

Environment variables/CLI option and default values:

| CLI option                    | Environment variable        | Default                                   |
|-------------------------------|-----------------------------|-------------------------------------------|
| `--access-key-id`             | `ECS_ACCESS_KEY_ID`         | -                                         |
| `--access-key-secret`         | `ECS_ACCESS_KEY_SECRET`     | -                                         |
| `--region-id`                 | `ECS_REGION_ID`             | `cn-beijing`                              |
| `--zone-id`                   | `ECS_ZONE_ID`               | -                                         |
| `--image-id`                  | `ECS_IMAGE_ID`              | `ubuntu1404_64_20G_aliaegis_20150325.vhd` |
| `--instance-type`             | -                           | `ecs.t1.small`                            |
| `--security-group-id`         | -                           | -                                         |
| `--internet-charge-type`      | -                           | `PayByTraffic`                            |
| `--root-password`             | -                           | `Password520`                             |
| `--io-optimized`              | -                           | `none`                                    |
| `--vswitch-id`                | -                           | -                                         |


## Development

### Build from Source
If you wish to work on aliyun Driver for Docker machine, you'll first need
[Go](http://www.golang.org) installed (version 1.5+ is required).
Make sure Go is properly installed, including setting up a [GOPATH](http://golang.org/doc/code.html#GOPATH).

Run these commands to build the plugin binary:

```bash
$ go get -d github.com/venusource/docker-machine-aliyun
$ cd $GOPATH/github.com/venusource/docker-machine-aliyun
$ make build
```

After the build is complete, `bin/docker-machine-driver-aliyun` binary will
be created. If you want to copy it to the `${GOPATH}/bin/`, run `make install`.
