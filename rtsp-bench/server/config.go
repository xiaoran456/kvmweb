package main

import (
	"bytes"
	"fmt"
	"sync"

	//win "github.com/n9e/win-collector/GoMiniblink/forms/windows/win32"
	//"github.com/n9e/win-collector/stra"
	//"github.com/n9e/win-collector/sys"
	//"github.com/n9e/win-collector/sys/identity"

	"github.com/pion/webrtc/v3"
	"github.com/spf13/viper"
	"github.com/toolkits/pkg/file"
)

/*
// Message
type Message struct {
	SeqID      uint64                    `json:"seqid"`
	Video      bool                      `json:"video"`
	Serial     bool                      `json:"serial"`
	SSH        bool                      `json:"ssh"`
	IceServer  string                    `json:"iceserver"`
	RtcSession webrtc.SessionDescription `json:"offer" mapstructure:"offer"`
}
*/
// Message
type Message struct {
	SeqID               uint64                    `json:"seqid"`
	Video               bool                      `json:"video"`
	Serial              bool                      `json:"serial"`
	SSH                 bool                      `json:"ssh"`
	IceServer           []string                  `json:"iceserver"`
	RtcSession          webrtc.SessionDescription `json:"offer" mapstructure:"offer"`
	VideoRtspServerAddr string                    `json:"rtspaddr"`
	Suuid               string                    `json:"suuid"` //视频流编号，浏览器可以通过预先获取，然后在使用时带过来，主要是提供一个选择分辨率和地址的作用，kvm的话内置4路分辨率，其余的如果是Onvif IPC类则通过Onvif协议在本地获取后通过mqtt传给浏览器，也可以考虑用探测软件实现探测后直接注册到夜莺平台，需要时前端到夜莺平台取
}
type ConfYaml struct {
	//Logger   logger.Config            `yaml:"logger"`
	//Identity IdentitySection `yaml:"identity"`
	//IP       IPSection       `yaml:"ip"`
	//Stra     StraSection     `yaml:"stra"`
	Enable enableSection `yaml:"enable"`
	Report reportSection `yaml:"report"`
	Mqtt   mqttSection   `yaml:"mqtt" mapstructure:"mqtt"`
	KVM    kvmSection    `yaml:"kvm" mapstructure:"kvm"`
}

type enableSection struct {
	Report bool `yaml:"report"`
}
type kvmSection struct {
	RTCConfig  webrtc.Configuration
	DeviceId   string
	Password   string
	ServerAddr string
	AudioSrc   string
	VideoSrc   string
	SSHHost    string
	SSHPort    int
}

type reportSection struct {
	Token    string            `yaml:"token"`
	Interval int               `yaml:"interval"`
	Cate     string            `yaml:"cate"`
	UniqKey  string            `yaml:"uniqkey"`
	SN       string            `yaml:"sn"`
	Fields   map[string]string `yaml:"fields"`
}

type mqttSection struct {
	SUBTOPIC      string `yaml:"subtopic" mapstructure:"subtopic"` //"topic1"
	PUBTOPIC      string `yaml:"pubtopic" mapstructure:"pubtopic"`
	QOS           byte   `yaml:"qos" mapstructure:"qos"`                     //1
	SERVERADDRESS string `yaml:"serveraddress" mapstructure:"serveraddress"` //= "tcp://mosquitto:1883"
	CLIENTID      string `yaml:"clientid" mapstructure:"clientid"`           //= "mqtt_subscriber"
	WRITETOLOG    bool   `yaml:"writelog" mapstructure:"writelog"`           //= true  // If true then received messages will be written to the console
	WRITETODISK   bool   `yaml:"writetodisk" mapstructure:"writetodisk"`     //= false // If true then received messages will be written to the file below
	OUTPUTFILE    string `yaml:"outputfile" mapstructure:"outputfile"`       //= "/binds/receivedMessages.txt"
	HEARTTIME     int    `yaml:"hearttime" mapstructure:"hearttime"`
	//	CommandLocalPath string `yam:"commanlocalpath"`
}

var (
	Config   *ConfYaml
	lock     = new(sync.RWMutex)
	Endpoint string
	Cwd      string
)

// Get configuration file
func Get() *ConfYaml {
	lock.RLock()
	defer lock.RUnlock()
	return Config
}

func Parse(conf string) error {
	bs, err := file.ReadBytes(conf)
	if err != nil {
		return fmt.Errorf("cannot read yml[%s]: %v", conf, err)
	}

	lock.Lock()
	defer lock.Unlock()

	viper.SetConfigType("yaml")
	//	viper.Set
	err = viper.ReadConfig(bytes.NewBuffer(bs))
	if err != nil {
		return fmt.Errorf("cannot read yml[%s]: %v", conf, err)
	}
	/*
		viper.SetDefault("worker", map[string]interface{}{
			"workerNum":    10,
			"queueSize":    1024000,
			"pushInterval": 5,
			"waitPush":     0,
		})

		viper.SetDefault("stra", map[string]interface{}{
			"enable":   true,
			"timeout":  1000,
			"interval": 10, //采集策略更新时间
			"portPath": "/home/n9e/etc/port",
			"procPath": "/home/n9e/etc/proc",
			"logPath":  "/home/n9e/etc/log",
			"api":      "/api/portal/collects/",
		})

		viper.SetDefault("sys", map[string]interface{}{
			"timeout":  1000, //请求超时时间
			"interval": 10,   //基础指标上报周期
			"plugin":   "/home/n9e/plugin",
		})
	*/
	err = viper.Unmarshal(&Config)
	if err != nil {
		return fmt.Errorf("Unmarshal %v", err)
	}

	return nil
}
