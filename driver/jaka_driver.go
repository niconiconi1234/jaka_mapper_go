package driver

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/hashicorp/go-multierror"
	"github.com/redis/go-redis/v9"
	"k8s.io/klog"
	"time"
	"strings"
)

func capitalizeAndRemoveSpace(str string) string {
    // 将字符串转换为单词列表
    words := strings.Split(str, " ")

    // 将每个单词的首字母大写
    for i, word := range words {
        words[i] = strings.ToUpper(word[:1]) + word[1:]
    }

    // 去除所有空格
    newStr := strings.Join(words, "")

    // 返回结果
    return newStr
}

type JakaDriver struct {
	Health         bool
	Status         string
	CurStep        string
	redisClient    *redis.Client
	stopDriverChan chan bool
}

type JakaProtocolConfig struct {
	ProtocolName string `yaml:"protocolName,omitempty"`
	ConfigData   struct {
		RedisHost     string `yaml:"redisHost,omitempty"`
		RedisPort     string `yaml:"redisPort,omitempty"`
		RedisPassword string `yaml:"redisPassword,omitempty"`
	} `yaml:"configData,omitempty"`
}

type JakaProtocolCommonConfig struct {
	CustomizedValues struct {
		RedisHost     string `yaml:"redisHost,omitempty"`
		RedisPort     string `yaml:"redisPort,omitempty"`
		RedisPassword string `yaml:"redisPassword,omitempty"`
	} `yaml:"customizedValues,omitempty"`
}

type JakaProtocolVisitorConfig struct {
	ProtocolName string `yaml:"protocolName,omitempty"`
	ConfigData   struct {
		PropertyName string `yaml:"propertyName,omitempty"`
	}
}

// InitDevice provide configmap parsing to specific protocols
func (d *JakaDriver) InitDevice(protocolCommon []byte) (err error) {
	success := true

	defer func() {
		if success {
			klog.Infof("Init jaka device success")
		} else {
			klog.Errorf("Init jaka device failed")
		}
	}()

	protocolCommonObj := JakaProtocolCommonConfig{}
	err = json.Unmarshal(protocolCommon, &protocolCommonObj)

	if err != nil {
		klog.Errorf("Unmarshal protocolCommon failed: %v", err)
		success = false
		return err
	}

	// 连接redis
	d.redisClient = redis.NewClient(&redis.Options{
		Addr:     protocolCommonObj.CustomizedValues.RedisHost + ":" + protocolCommonObj.CustomizedValues.RedisPort,
		Password: protocolCommonObj.CustomizedValues.RedisPassword,
		DB:       0,
	})

	// 测试redis连接
	_, err = d.redisClient.Ping(context.Background()).Result()
	if err != nil {
		klog.Errorf("Redis connect failed: %v", err)
		success = false
		return err
	}

	go func() {
		for {
			select {
			case <-d.stopDriverChan:
				klog.Infof("Stop jaka driver")
				return
			default:
				err = d.readJakaInfoFromRedis()
				if err != nil {
					klog.Errorf("Read jaka info from redis failed: %v", err)
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()

	return nil
}

// ReadDeviceData  is an interface that reads data from a specific device, data is a type of string
func (d *JakaDriver) ReadDeviceData(protocolCommon []byte, visitor []byte, protocol []byte) (data interface{}, err error) {
	visitorConfig := JakaProtocolVisitorConfig{}
	err = json.Unmarshal(visitor, &visitorConfig)

	if err != nil {
		klog.Errorf("Unmarshal visitor failed: %v", err)
		return nil, err
	}

	switch visitorConfig.ConfigData.PropertyName {
	case "health":
		return d.Health, nil
	case "status":
		if d.Health {
			return d.Status, nil
		} else {
			return "DISCONNECTED", nil
		}
	case "curStep":
		if d.Health {
			return capitalizeAndRemoveSpace(d.CurStep), nil
		} else {
			return "DISCONNECTED", nil
		}
	}

	return nil, errors.New("visitor property name not found")
}

// WriteDeviceData is an interface that write data to a specific device, data's DataType is Consistent with configmap
func (d *JakaDriver) WriteDeviceData(data interface{}, protocolCommon []byte, visitor []byte, protocol []byte) (err error) {
	klog.Error("Jaka driver does not support write data")
	return errors.New("jaka driver does not support write data")
}

// StopDevice is an interface to stop all devices
func (d *JakaDriver) StopDevice() (err error) {
	d.stopDriverChan <- true
	return nil
}

// GetDeviceStatus is an interface to get the device status true is OK , false is DISCONNECTED
func (d *JakaDriver) GetDeviceStatus(protocolCommon []byte, visitor []byte, protocol []byte) (status bool) {
	return d.Health
}

func (d *JakaDriver) readJakaInfoFromRedis() (err error) {
	REDIS_JAKA_HEALTH_KEY := "JAKA:HEALTH"
	REDIS_JAKA_STATUS_KEY := "JAKA:STATUS"
	REDIS_JAKA_CUR_STEP_KEY := "JAKA:CUR_STEP"

	// 读取健康状态, 状态, 当前步骤
	health, err1 := d.redisClient.Get(context.Background(), REDIS_JAKA_HEALTH_KEY).Result()
	status, err2 := d.redisClient.Get(context.Background(), REDIS_JAKA_STATUS_KEY).Result()
	curStep, err3 := d.redisClient.Get(context.Background(), REDIS_JAKA_CUR_STEP_KEY).Result()

	if err1 != nil {
		klog.Errorf("Read jaka health from redis failed: %v", err1)
		d.Health = false
		return err1
	}

	totalErr := multierror.Append(err1, err2, err3).ErrorOrNil()
	if totalErr != nil {
		klog.Errorf("Read jaka info from redis failed: %v", totalErr)
		return totalErr
	}

	d.Health = health == "1"
	d.Status = status
	d.CurStep = curStep

	return nil
}
