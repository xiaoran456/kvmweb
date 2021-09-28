package kvm

// Connect to the broker, subscribe, and write messages received to a file
import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	//"github.com/didi/nightingale/src/modules/agent/config"
	//"github.com/didi/nightingale/src/modules/agent/wol"
	"github.com/pion/rtsp-bench/server/config"
	"github.com/pion/rtsp-bench/server/wol"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	enc "github.com/pion/rtsp-bench/server/signal"
)

/*
const (
	TOPIC         = "topic1"
	QOS           = 1
	SERVERADDRESS = "tcp://mosquitto:1883"
	CLIENTID      = "mqtt_subscriber"

	WRITETOLOG  = true  // If true then received messages will be written to the console
	WRITETODISK = false // If true then received messages will be written to the file below

	OUTPUTFILE = "/binds/receivedMessages.txt"
)
*/
// handler is a simple struct that provides a function to be called when a message is received. The message is parsed
// and the count followed by the raw message is written to the file (this makes it easier to sort the file)
type handler struct {
	f *os.File
}

//var  mqttclient mqtt.Client
var (
	msgChans chan PublishMsg //prompb.WriteRequest //multi node one chan
)

type heartmsg struct {
	Count uint64
}

func NewHandler() *handler {
	var f *os.File
	if config.Config.Mqtt.WRITETODISK {
		var err error
		f, err = os.Create(config.Config.Mqtt.OUTPUTFILE)
		if err != nil {
			panic(err)
		}
	}
	return &handler{f: f}
}

// Close closes the file
func (o *handler) Close() {
	if o.f != nil {
		if err := o.f.Close(); err != nil {
			fmt.Printf("ERROR closing file: %s", err)
		}
		o.f = nil
	}
}

// handle is called when a message is received
func (o *handler) handle(client mqtt.Client, msg mqtt.Message) {
	// We extract the count and write that out first to simplify checking for missing values
	var m Message
	var resp Session
	if err := json.Unmarshal(msg.Payload(), &resp); err != nil {
		fmt.Printf("Message could not be parsed (%s): %s", msg.Payload(), err)
		return
	}
	fmt.Println(resp)
	switch resp.Type {
	case CMDMSG_OFFER:
		enc.Decode(resp.Data, &m)
		Notice(m)
	case CMDMSG_DISC:
		var devcmd DiscoveryCmd
		enc.Decode(resp.Data, &devcmd)
		DiscoveryDev(&devcmd)
	case CMDMSG_WAKE:
		var fing Fing
		enc.Decode(resp.Data, &fing)
		wakemac(fing)
	case CMDMSG_UPDATE:
		var newver *versionUpdate
		GetUpdateMyself(newver)
	case CMDMSG_MR2:
		var mr2info Mr2Msg
		enc.Decode(resp.Data, &mr2info)
		Mr2HostPort(&mr2info)
	}
}
func Mr2HostPort(mr2info *Mr2Msg) {
	arg := fmt.Sprintf("client -s %s -p %s -P %d -c %s", mr2info.ServerAddr, mr2info.Password, mr2info.ExposePort, mr2info.ExposeAddr)
	fmt.Println("mr2", arg)
	err := fmt.Errorf("")
	//err := sys.CmdRun("./mr2", arg)
	if err != nil {
		CmdFeedBack(CMDMSG_MR2, 0, err.Error(), time.Now().String())
		return
	} else {
		CmdFeedBack(CMDMSG_MR2, 1, "成功", time.Now().String())
	}
}
func wakemac(fing Fing) {
	for _, v := range fing.Devices {
		wol.Wake(v.Mac, "", "", "")
	}
}
func DiscoveryDev(devcmd *DiscoveryCmd) {
	go func() {
		switch devcmd.DevType {
		case DEVICE_IP:
			dev := DiscoveryDevice()
			req := &Session{}
			req.Type = "discoveryrsp"
			req.DeviceId = "kvm1"
			req.Data = enc.Encode(dev) //enc.Encode(answer)
			answermsg := PublishMsg{
				Topic: "discoveryrsp",
				Msg:   req,
			}
			fmt.Println("discoveryrsp", answermsg)
			SendMsg(answermsg) //response)
		case DEVICE_ONVIF:
		case DEVICE_SNMP:
		case DEVICE_MODBUS:
		case DEVICE_BACNET:
		case DEVICE_CAN:
		case DEVICE_UPCA:
		}
	}()
}
func CmdFeedBack(cmdstr string, status int, err string, sid string) {

	resp := ResponseMsg{
		Cmdstr: cmdstr,
		Status: status,
		Err:    err,
		Sid:    sid,
	}
	req := &Session{}
	req.Type = "cmdFeedback"
	req.DeviceId = "kvm1"
	req.Data = enc.Encode(resp) //enc.Encode(answer)
	answermsg := PublishMsg{
		Topic: "cmdFeedback",
		Msg:   req,
	}
	fmt.Println("cmdFeedback", answermsg)
	SendMsg(answermsg) //response)
}
func GetCurrentPath() string {
	getwd, err := os.Getwd()
	if err != nil {
		fmt.Print(err.Error())
	} else {
		fmt.Print(getwd)
	}
	return getwd
}
func GetUpdateMyself(newver *versionUpdate) {
	if newver.ForceUpdate == 1 {
		if newver.DownLoadUrl != "" {

			go func() {
				filepath := GetCurrentPath() + "/" + newver.Version
				fileext := ".zip"
				filename, err := DownloadFile(filepath, newver.DownLoadUrl, fileext)
				if err != nil {
					CmdFeedBack(CMDMSG_UPDATE, 0, err.Error(), "1")

				} else {
					fmt.Println("Download Finished")
					CmdFeedBack(CMDMSG_UPDATE, 1, "Download Finished", "1")
					if IsZip(filename) {
						err = Unzip(filename, filepath)
						if err != nil {
							CmdFeedBack(CMDMSG_UPDATE, 0, err.Error(), "1")

						} else {
							CmdFeedBack(CMDMSG_UPDATE, 1, "zip ok", "1")
						}
					}
					//以版本号建立新目录并解压包
					//后期写更新配置文件告诉Process守护进程需要更新
					//并立即退出或者按某种策略更新
					//守护进程在判断到需要更新软件时就按更新配置文件的新路径执行程序
				}
			}()
		}
	}
}

/*
func SendMsgAnswer(msg Answer) {

	msgChans <- msg
	fmt.Print("SendMsg OK")
	//mqttclient.Publish(Config.Mqtt.PUBTOPIC+"/"+Config.Report.SN, Config.Mqtt.QOS, false, msg)
}
*/
func SendMsg(msg PublishMsg) {

	msgChans <- msg
	fmt.Print("SendMsg OK")
	//mqttclient.Publish(Config.Mqtt.PUBTOPIC+"/"+Config.Report.SN, Config.Mqtt.QOS, false, msg)
}

func StartMqtt() {
	// Enable logging by uncommenting the below
	// mqtt.ERROR = log.New(os.Stdout, "[ERROR] ", 0)
	// mqtt.CRITICAL = log.New(os.Stdout, "[CRITICAL] ", 0)
	// mqtt.WARN = log.New(os.Stdout, "[WARN] ", 0)
	// mqtt.DEBUG = log.New(os.Stdout, "[DEBUG] ", 0)

	// Create a handler that will deal with incoming messages
	h := NewHandler()
	defer h.Close()
	msgChans = make(chan PublishMsg, 10)
	// Now we establish the connection to the mqtt broker
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.Config.Mqtt.SERVERADDRESS)
	opts.SetClientID(config.Config.Mqtt.CLIENTID)

	opts.ConnectTimeout = time.Second // Minimal delays on connect
	opts.WriteTimeout = time.Second   // Minimal delays on writes
	opts.KeepAlive = 30               // Keepalive every 10 seconds so we quickly detect network outages
	opts.PingTimeout = time.Second    // local broker so response should be quick

	// Automate connection management (will keep trying to connect and will reconnect if network drops)
	opts.ConnectRetry = true
	opts.AutoReconnect = true

	// If using QOS2 and CleanSession = FALSE then it is possible that we will receive messages on topics that we
	// have not subscribed to here (if they were previously subscribed to they are part of the session and survive
	// disconnect/reconnect). Adding a DefaultPublishHandler lets us detect this.
	opts.DefaultPublishHandler = func(_ mqtt.Client, msg mqtt.Message) {
		fmt.Printf("UNEXPECTED MESSAGE: %s\n", msg)
	}

	// Log events
	opts.OnConnectionLost = func(cl mqtt.Client, err error) {
		fmt.Println("connection lost")
	}

	opts.OnConnect = func(c mqtt.Client) {
		fmt.Println("connection established")

		// Establish the subscription - doing this here means that it willSUB happen every time a connection is established
		// (useful if opts.CleanSession is TRUE or the broker does not reliably store session data)
		t := c.Subscribe(config.Config.Mqtt.SUBTOPIC, config.Config.Mqtt.QOS, h.handle)
		// the connection handler is called in a goroutine so blocking here would hot cause an issue. However as blocking
		// in other handlers does cause problems its best to just assume we should not block
		go func() {
			_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
			if t.Error() != nil {
				fmt.Printf("ERROR SUBSCRIBING: %s\n", t.Error())
			} else {
				fmt.Println("subscribed to: ", config.Config.Mqtt.SUBTOPIC)
			}
		}()
	}
	opts.OnReconnecting = func(mqtt.Client, *mqtt.ClientOptions) {
		fmt.Println("attempting to reconnect")
	}

	//
	// Connect to the broker
	//
	client := mqtt.NewClient(opts)

	// If using QOS2 and CleanSession = FALSE then messages may be transmitted to us before the subscribe completes.
	// Adding routes prior to connecting is a way of ensuring that these messages are processed
	client.AddRoute(config.Config.Mqtt.SUBTOPIC, h.handle)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Println("Connection is up")
	done := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		var count uint64
		for {

			select {
			case data := <-msgChans:
				msg, err := json.Marshal(data.Msg)
				if err != nil {
					panic(err)
				}
				//t := client.Publish(Config.Mqtt.PUBTOPIC+"/"+Config.Report.SN, Config.Mqtt.QOS, false, msg)
				t := client.Publish(config.Config.Mqtt.PUBTOPIC+"/"+data.Topic, config.Config.Mqtt.QOS, false, msg)
				go func() {
					_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
					if t.Error() != nil {
						fmt.Printf("msg PUBLISHING: %s\n", t.Error().Error())
					} else {
						//fmt.Println("msg PUBLISHING:", msg)
					}
				}()
			case <-time.After(time.Second * time.Duration(config.Config.Mqtt.HEARTTIME)):
				req := &Session{}
				req.Type = "heart"
				req.DeviceId = config.Config.Mqtt.CLIENTID //"kvm1"
				count += 1
				msg, err := json.Marshal(heartmsg{Count: count})
				if err != nil {
					panic(err)
				}
				req.Data = enc.Encode(msg)
				//data := signal.Encode(*peerConnection.LocalDescription())
				answermsg := PublishMsg{
					Topic: "heart",
					Msg:   req,
				}
				msg, err = json.Marshal(answermsg.Msg)
				if err != nil {
					panic(err)
				}
				t := client.Publish(config.Config.Mqtt.PUBTOPIC+"/"+answermsg.Topic, config.Config.Mqtt.QOS, false, msg)
				// Handle the token in a go routine so this loop keeps sending messages regardless of delivery status
				go func() {
					_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
					if t.Error() != nil {
						fmt.Printf("ERROR PUBLISHING: %s\n", t.Error().Error())
					} else {
						//fmt.Println("HEART PUBLISHING: ", msg)
					}
				}()
			case <-done:
				fmt.Println("publisher done")
				wg.Done()
				return
			}
		}
	}()
	// Messages will be delivered asynchronously so we just need to wait for a signal to shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	signal.Notify(sig, syscall.SIGTERM)

	<-sig
	fmt.Println("signal caught - exiting")
	client.Disconnect(1000)
	fmt.Println("shutdown complete")
}

/*
// Connect to the broker and publish a message periodically

const (
	TOPIC         = "topic1"
	QOS           = 1
	SERVERADDRESS = "tcp://mosquitto:1883"
	DELAY         = time.Second
	CLIENTID      = "mqtt_publisher"
)

func main() {
	// Enable logging by uncommenting the below
	// mqtt.ERROR = log.New(os.Stdout, "[ERROR] ", 0)
	// mqtt.CRITICAL = log.New(os.Stdout, "[CRITICAL] ", 0)
	// mqtt.WARN = log.New(os.Stdout, "[WARN]  ", 0)
	// mqtt.DEBUG = log.New(os.Stdout, "[DEBUG] ", 0)
	opts := mqtt.NewClientOptions()
	opts.AddBroker(SERVERADDRESS)
	opts.SetClientID(CLIENTID)

	opts.ConnectTimeout = time.Second // Minimal delays on connect
	opts.WriteTimeout = time.Second   // Minimal delays on writes
	opts.KeepAlive = 10               // Keepalive every 10 seconds so we quickly detect network outages
	opts.PingTimeout = time.Second    // local broker so response should be quick

	// Automate connection management (will keep trying to connect and will reconnect if network drops)
	opts.ConnectRetry = true
	opts.AutoReconnect = true

	// Log events
	opts.OnConnectionLost = func(cl mqtt.Client, err error) {
		fmt.Println("connection lost")
	}
	opts.OnConnect = func(mqtt.Client) {
		fmt.Println("connection established")
	}
	opts.OnReconnecting = func(mqtt.Client, *mqtt.ClientOptions) {
		fmt.Println("attempting to reconnect")
	}

	//
	// Connect to the broker
	//
	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Println("Connection is up")

	//
	// Publish messages until we receive a signal
	//
	done := make(chan struct{})
	var wg sync.WaitGroup

	// The message could be anything; lets make it JSON containing a simple count (makes it simpler to track the messages)
	type msg struct {
		Count uint64
	}

	wg.Add(1)
	go func() {
		var count uint64
		for {
			select {
			case <-time.After(DELAY):
				count += 1
				msg, err := json.Marshal(msg{Count: count})
				if err != nil {
					panic(err)
				}

				t := client.Publish(TOPIC, QOS, false, msg)
				// Handle the token in a go routine so this loop keeps sending messages regardless of delivery status
				go func() {
					_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
					if t.Error() != nil {
						fmt.Printf("ERROR PUBLISHING: %s\n", err)
					}
				}()
			case <-done:
				fmt.Println("publisher done")
				wg.Done()
				return
			}
		}
	}()

	// Wait for a signal before exiting
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	signal.Notify(sig, syscall.SIGTERM)

	<-sig
	fmt.Println("signal caught - exiting")

	close(done)
	wg.Wait()
	fmt.Println("shutdown complete")
}
*/
