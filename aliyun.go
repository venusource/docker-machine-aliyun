package aliyun

import (
	"fmt"
	"github.com/ChangjunZhao/aliyun-api-golang/ecs"
	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnflag"
	"github.com/docker/machine/libmachine/ssh"
	"github.com/docker/machine/libmachine/state"
	"github.com/hypersleep/easyssh"
	"net"
	"time"
)

const (
	driverName                     = "aliyun"
	defaultRegionId                = "cn-beijing"
	defaultImageId                 = "ubuntu1404_64_20G_aliaegis_20150325.vhd"
	defaultInstanceType            = "ecs.t1.small"
	defaultInternetChargeType      = "PayByTraffic"
	defaultInternetMaxBandwidthIn  = "10"
	defaultInternetMaxBandwidthOut = "10"
	defaultPassword                = "Password520"
	defaultIoOptimized             = "none"
)

type Driver struct {
	*drivers.BaseDriver
	InstanceID          string
	AccessKeyID         string
	AccessKeySecret     string
	RegionId            string
	ZoneId              string
	ImageId             string
	InstanceType        string
	SecurityGroupId     string
	InternetChargeType  string
	Password            string
	IoOptimized         string
	VSwitchId           string
	IsTempSecurityGroup bool
}

func NewDriver(hostName, storePath string) *Driver {
	return &Driver{
		BaseDriver: &drivers.BaseDriver{
			MachineName: hostName,
			StorePath:   storePath,
		},
	}
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

// DriverName returns the name of the driver
func (d *Driver) DriverName() string {
	return driverName
}

func (d *Driver) GetURL() (string, error) {
	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}
	if ip == "" {
		return "", nil
	}
	return fmt.Sprintf("tcp://%s", net.JoinHostPort(ip, "2376")), nil
}

func (d *Driver) GetIP() (string, error) {

	c := ecs.NewClient(
		d.AccessKeyID,
		d.AccessKeySecret,
	)

	instance, err := c.DescribeInstanceAttribute(d.RegionId, d.InstanceID)

	if err != nil {
		return "", fmt.Errorf("No IP found for the machine")
	} else {
		return instance.PublicIpAddress.IpAddress[0], nil
	}

	return "", fmt.Errorf("No IP found for the machine")
}

func (d *Driver) GetState() (state.State, error) {
	log.WithField("MachineId", d.InstanceID).Debug("Get status for aliyun instance...")
	c := ecs.NewClient(
		d.AccessKeyID,
		d.AccessKeySecret,
	)
	if instance, err := c.DescribeInstanceAttribute(d.RegionId, d.InstanceID); err != nil {
		return state.None, err
	} else {
		switch instance.Status {
		case "Running":
			return state.Running, nil
		case "Stopping":
			return state.Paused, nil
		case "Stopped":
			return state.Stopped, nil
		case "Starting":
			return state.Starting, nil
		case "ERROR":
			return state.Error, nil
		}
	}
	return state.None, nil
}

func (d *Driver) Create() error {
	if d.SecurityGroupId == "" {
		log.WithField("driver", d.DriverName()).Info("Begin create SecurityGroup...")
		err := d.createSecurityGroup()
		if err != nil {
			return fmt.Errorf(errorOnCreateSecurityGroup, err.Error())
		}
	}

	log.WithField("driver", d.DriverName()).Info("Begin create aliyun instance...")
	d.createSSHKey()

	c := ecs.NewClient(
		d.AccessKeyID,
		d.AccessKeySecret,
	)
	request := &ecs.CreateInstanceRequest{
		RegionId:                d.RegionId,
		ImageId:                 d.ImageId,
		InstanceType:            d.InstanceType,
		SecurityGroupId:         d.SecurityGroupId,
		Password:                d.Password,
		InternetChargeType:      d.InternetChargeType,
		InternetMaxBandwidthIn:  defaultInternetMaxBandwidthIn,
		InternetMaxBandwidthOut: defaultInternetMaxBandwidthOut,
		VSwitchId:               d.VSwitchId,
	}
	response, err := c.CreateInstanceByRequest(request)
	if err != nil {
		return fmt.Errorf(errorOnCreateMachine, err.Error())
	} else {
		d.InstanceID = response.InstanceId
		//如果分配公网IP失败，那么删除虚拟机
		if publicIPAddress, err := c.AllocatePublicIpAddress(d.InstanceID); err != nil {
			if err := c.DeleteInstance(d.InstanceID); err != nil {
				return fmt.Errorf(errorOnRollback, err.Error())
			}
			return fmt.Errorf(errorOnAllocatePublicIpAddress, err.Error())
		} else {
			d.IPAddress = publicIPAddress
		}
		//如果无法启动实例，那么删除虚拟机，注：目前不知道阿里云是否会出现这种情况
		if err := c.StartInstance(d.InstanceID); err != nil {
			if err := c.DeleteInstance(d.InstanceID); err != nil {
				return fmt.Errorf(errorOnRollback, err.Error())
			}
			return fmt.Errorf(errorOnStartMachine, err.Error())
		}
	}

	//等待启动完成，如果启动失败，删除虚拟机
	if err := d.waitForInstanceActive(); err != nil {
		if err := c.DeleteInstance(d.InstanceID); err != nil {
			return fmt.Errorf(errorOnRollback, err.Error())
		}
		return err
	}
	//上传公钥
	if err := d.pushSSHkeyToServer(); err != nil {
		if err := c.DeleteInstance(d.InstanceID); err != nil {
			return fmt.Errorf(errorOnRollback, err.Error())
		}
		return err
	}

	return nil
}

func (d *Driver) deleteSecurityGroup() {
	c := ecs.NewClient(
		d.AccessKeyID,
		d.AccessKeySecret,
	)
	_, err := c.DeleteSecurityGroup(&ecs.DeleteSecurityGroupRequest{RegionId: d.RegionId, SecurityGroupId: d.SecurityGroupId})
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println("删除安全组失败，请登录阿里云管理控制台手动删除")
	}
}

func (d *Driver) createSecurityGroup() error {
	c := ecs.NewClient(
		d.AccessKeyID,
		d.AccessKeySecret,
	)
	req := &ecs.CreateSecurityGroupRequest{RegionId: d.RegionId, SecurityGroupName: "Docker_Machine"}
	response, err := c.CreateSecurityGroup(req)
	if err != nil {
		return fmt.Errorf(errorOnCreateMachine, err.Error())
	} else {
		d.SecurityGroupId = response.SecurityGroupId
		//22端口
		_, err := c.AuthorizeSecurityGroup(d.CreateAuthorizeSecurityGroupRequest(d.SecurityGroupId, "22/22"))
		if err != nil {
			d.deleteSecurityGroup()
			return fmt.Errorf(err.Error())
		}

		//2376端口
		_, err = c.AuthorizeSecurityGroup(d.CreateAuthorizeSecurityGroupRequest(d.SecurityGroupId, "2376/2376"))
		if err != nil {
			d.deleteSecurityGroup()
			return fmt.Errorf(err.Error())
		}

		if d.SwarmMaster {
			//3376端口
			_, err = c.AuthorizeSecurityGroup(d.CreateAuthorizeSecurityGroupRequest(d.SecurityGroupId, "3376/3376"))
			if err != nil {
				d.deleteSecurityGroup()
				return fmt.Errorf(err.Error())
			}
		}

	}
	d.IsTempSecurityGroup = true
	return nil
}

func (d *Driver) CreateAuthorizeSecurityGroupRequest(securityGroupId string, portRange string) *ecs.AuthorizeSecurityGroupRequest {
	return &ecs.AuthorizeSecurityGroupRequest{SecurityGroupId: securityGroupId, RegionId: d.RegionId, IpProtocol: "tcp", PortRange: portRange, SourceCidrIp: "0.0.0.0/0"}
}

/**
等待服务器启动
**/
func (d *Driver) waitForInstanceActive() error {
	log.WithField("MachineId", d.InstanceID).Debug("Waiting for the aliyun instance to be ACTIVE...")
	for {

		if cstate, _ := d.GetState(); cstate == state.Running {
			break
		}
		time.Sleep(10 * time.Second)
	}
	return nil
}

func (d *Driver) Start() error {
	log.WithField("InstanceID", d.InstanceID).Info("Starting aliyun instance...")
	c := ecs.NewClient(
		d.AccessKeyID,
		d.AccessKeySecret,
	)
	if err := c.StartInstance(d.InstanceID); err != nil {
		return fmt.Errorf(errorOnStartMachine, err.Error())
	}
	return nil
}

func (d *Driver) Stop() error {
	log.WithField("InstanceID", d.InstanceID).Info("Stopping aliyun instance...")
	c := ecs.NewClient(
		d.AccessKeyID,
		d.AccessKeySecret,
	)

	if err := c.StopInstance(d.InstanceID, "false"); err != nil {
		return fmt.Errorf(errorOnStopMachine, err.Error())
	}
	return nil
}

func (d *Driver) Remove() error {
	log.WithField("InstanceID", d.InstanceID).Debug("deleting instance...")
	log.Info("Deleting aliyun instance...")
	c := ecs.NewClient(
		d.AccessKeyID,
		d.AccessKeySecret,
	)
	if cstate, _ := d.GetState(); cstate == state.Running {

		if err := c.StopInstance(d.InstanceID, "true"); err != nil {
			return fmt.Errorf(errorOnStopMachine, err.Error())
		}
		//等待虚拟机停机
		for {
			if cstate, _ = d.GetState(); cstate == state.Stopped {
				break
			}
			time.Sleep(3 * time.Second)
		}
	}
	if err := c.DeleteInstance(d.InstanceID); err != nil {
		return fmt.Errorf(errorOnRemoveMachine, err.Error())
	}

	if d.IsTempSecurityGroup {
		for {
			if cstate, _ := d.GetState(); cstate == state.None {
				d.deleteSecurityGroup()
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
	return nil
}

func (d *Driver) Restart() error {
	log.WithField("InstanceID", d.InstanceID).Info("Restarting Aliyun instance...")
	c := ecs.NewClient(
		d.AccessKeyID,
		d.AccessKeySecret,
	)
	if err := c.RebootInstance(d.InstanceID, "false"); err != nil {
		return fmt.Errorf(errorOnRestartMachine, err.Error())
	}
	return nil
}

func (d *Driver) Kill() error {
	return d.Stop()
}

const (
	errorMandatoryEnvOrOption      string = "%s must be specified either using the environment variable %s or the CLI option %s"
	errorOnCreateMachine           string = "can not create machine: %s"
	errorOnStartMachine            string = "can not start machine: %s"
	errorOnStopMachine             string = "can not stop machine: %s"
	errorOnRemoveMachine           string = "can not remove machine: %s"
	errorOnRestartMachine          string = "can not restart machine: %s"
	errorOnAllocatePublicIpAddress string = "can not AllocatePublicIpAddress: %s"
	errorOnRollback                string = "程序不能自动删除虚拟机，回滚失败，请手动删除: %s"
	errorOnCreateSecurityGroup     string = "Can not create SecurityGroup"
)

func (d *Driver) checkConfig() error {
	/*
		if d.SecurityGroupId == "" {
			return fmt.Errorf(errorMandatoryEnvOrOption, "Aliyun安全组ID", "SECURITY_GROUP_ID", "--security-group-id")
		}
	*/
	if d.AccessKeyID == "" {
		return fmt.Errorf(errorMandatoryEnvOrOption, "Aliyun Access Key Id", "ACCESS_KEY_ID", "--access-key-id")
	}
	if d.AccessKeySecret == "" {
		return fmt.Errorf(errorMandatoryEnvOrOption, "Aliyun Access Key Secret", "ACCESS_KEY_SECRET", "--access-key-secret")
	}
	return nil
}

func (d *Driver) createSSHKey() error {
	log.Debug("Creating Key Pair...")
	if err := ssh.GenerateSSHKey(d.GetSSHKeyPath()); err != nil {
		return err
	}
	return nil
}

func (d *Driver) publicSSHKeyPath() string {
	return d.GetSSHKeyPath() + ".pub"
}

func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		mcnflag.StringFlag{
			EnvVar: "ECS_ACCESS_KEY_ID",
			Name:   "access-key-id",
			Usage:  "Aliyun Access Key Id",
			Value:  "",
		},
		mcnflag.StringFlag{
			EnvVar: "ECS_ACCESS_KEY_SECRET",
			Name:   "access-key-secret",
			Usage:  "Aliyun Access Key Secret",
		},
		mcnflag.StringFlag{
			EnvVar: "ECS_REGION_ID",
			Name:   "region-id",
			Usage:  "实例所属的 Region ID",
			Value:  defaultRegionId,
		},
		mcnflag.StringFlag{
			EnvVar: "ECS_ZONE_ID",
			Name:   "zone-id",
			Usage:  "实例所属的可用区编号，空表示由系统选择，默认值：空。",
			Value:  "",
		},
		mcnflag.StringFlag{
			EnvVar: "ECS_IMAGE_ID",
			Name:   "image-id",
			Usage:  "镜像文件 ID，表示启动实例时选择的镜像资源",
			Value:  defaultImageId,
		},
		mcnflag.StringFlag{
			Name:  "instance-type",
			Usage: "实例的资源规则",
			Value: defaultInstanceType,
		},
		mcnflag.StringFlag{
			EnvVar: "SECURITY_GROUP_ID",
			Name:   "security-group-id",
			Usage:  "指定新创建实例所属于的安全组代码，同一个安全组内的实例之间可以互相访问。",
			Value:  "",
		},
		mcnflag.StringFlag{
			Name:  "internet-charge-type",
			Usage: "网络计费类型:PayByBandwidth/PayByTraffic",
			Value: defaultInternetChargeType,
		},
		mcnflag.StringFlag{
			Name:  "root-password",
			Usage: "root用户密码",
			Value: defaultPassword,
		},
		mcnflag.StringFlag{
			Name:  "io-optimized",
			Usage: "IO优化,none：非IO优化,optimized：IO优化",
			Value: defaultIoOptimized,
		},
		mcnflag.StringFlag{
			Name:  "vswitch-id",
			Usage: "vpc vswitch id",
		},
	}
}

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	d.AccessKeyID = flags.String("access-key-id")
	d.AccessKeySecret = flags.String("access-key-secret")
	d.RegionId = flags.String("region-id")
	d.ZoneId = flags.String("zone-id")
	d.ImageId = flags.String("image-id")
	d.InstanceType = flags.String("instance-type")
	d.SecurityGroupId = flags.String("security-group-id")
	d.InternetChargeType = flags.String("internet-charge-type")
	d.Password = flags.String("root-password")
	d.IoOptimized = flags.String("io-optimized")
	d.VSwitchId = flags.String("vswitch-id")
	d.SwarmMaster = flags.Bool("swarm-master")
	d.SwarmHost = flags.String("swarm-host")
	d.SwarmDiscovery = flags.String("swarm-discovery")
	return d.checkConfig()
}

/**
目前阿里云不支持启动虚拟机注入Key，只能通过SSH先将Key写入服务器
**/
func (d *Driver) pushSSHkeyToServer() error {
	ssh := &easyssh.MakeConfig{
		User:     "root",
		Server:   d.IPAddress,
		Password: d.Password,
		Port:     "22",
	}
	if err := ssh.Scp(d.publicSSHKeyPath()); err != nil {
		return err
	}
	if _, err := ssh.Run("mkdir .ssh; cat id_rsa.pub >> .ssh/authorized_keys; chmod 600 .ssh/authorized_keys; rm id_rsa.pub"); err != nil {
		return err
	}
	return nil
}
