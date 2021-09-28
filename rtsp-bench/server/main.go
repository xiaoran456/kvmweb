package main

//package test

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/format/rtsp"
	"github.com/gin-gonic/gin"
	"github.com/pion/rtsp-bench/server/config"
	"github.com/pion/rtsp-bench/server/kvm"
	enc "github.com/pion/rtsp-bench/server/signal"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/shirou/gopsutil/cpu"
	"github.com/toolkits/pkg/file"
)

var (
	vers                *bool
	help                *bool
	conf                *string
	outboundVideoTrack  *webrtc.TrackLocalStaticSample
	peerConnectionCount int64
	Version             = "v0.1.0"
	//outboundVideoTrack  *webrtc.TrackLocalStaticSample
	//peerConnectionCount int64
)

// Generate CSV with columns of timestamp, peerConnectionCount, and cpuUsage
func reportBuilder() {
	file, err := os.OpenFile("report.csv", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}

	if _, err := file.WriteString("timestamp, peerConnectionCount, cpuUsage\n"); err != nil {
		panic(err)
	}

	for range time.NewTicker(3 * time.Second).C {
		usage, err := cpu.Percent(0, false)
		if err != nil {
			panic(err)
		} else if len(usage) != 1 {
			panic(fmt.Sprintf("CPU Usage results should have 1 sample, have %d", len(usage)))
		}
		if _, err = file.WriteString(fmt.Sprintf("%s, %d, %f\n", time.Now().Format(time.RFC3339), atomic.LoadInt64(&peerConnectionCount), usage[0])); err != nil {
			panic(err)
		}
	}
}

// HTTP Handler that accepts an Offer and returns an Answer
// adds outboundVideoTrack to PeerConnection
func doSignaling(w http.ResponseWriter, r *http.Request) {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		if connectionState == webrtc.ICEConnectionStateDisconnected {
			atomic.AddInt64(&peerConnectionCount, -1)
			if err := peerConnection.Close(); err != nil {
				panic(err)
			}
		} else if connectionState == webrtc.ICEConnectionStateConnected {
			atomic.AddInt64(&peerConnectionCount, 1)
		}
	})

	if _, err = peerConnection.AddTrack(outboundVideoTrack); err != nil {
		panic(err)
	}

	var offer webrtc.SessionDescription
	if err = json.NewDecoder(r.Body).Decode(&offer); err != nil {
		panic(err)
	}

	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	gatherCompletePromise := webrtc.GatheringCompletePromise(peerConnection)

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	<-gatherCompletePromise

	response, err := json.Marshal(*peerConnection.LocalDescription())
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(response); err != nil {
		panic(err)
	}
}

func init() {
	vers = flag.Bool("v", false, "display the version.")
	help = flag.Bool("h", false, "print this help.")
	conf = flag.String("f", "", "specify configuration file.")
	flag.Parse()

	if *vers {
		fmt.Println("version:", Version)
		os.Exit(0)
	}

	if *help {
		flag.Usage()
		os.Exit(0)
	}
}

// auto detect configuration file
func aconf() {
	if *conf != "" && file.IsExist(*conf) {
		return
	}

	*conf = "etc/kvmagent.local.yml"
	if file.IsExist(*conf) {
		return
	}

	*conf = "etc/kvmagent.yml"
	if file.IsExist(*conf) {
		return
	}

	fmt.Println("no configuration file for collector")
	os.Exit(1)
}

// parse configuration file
func pconf() {

	if err := Parse(*conf); err != nil {
		fmt.Println("cannot parse configuration file:", err)
		os.Exit(1)
	}

}

var rtspsrcmap map[string]*webrtc.TrackLocalStaticSample

func startKVMClient() {
	//uiTest()
	aconf()
	pconf()
	var err error
	rtspsrcmap = make(map[string]*webrtc.TrackLocalStaticSample)
	outboundVideoTrack, err = webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType: "video/h264",
	}, "pion-rtsp", "pion-rtsp")
	if err != nil {
		panic(err)
	}
	rtspsrcmap[kvmrtspURL] = outboundVideoTrack
	go rtspConsumer()
	go StartMqtt()
	//go screen.StartScreentShot()
}
func main1() {
	parseConf()
	kvm.StartKvmAgent()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	signal.Notify(sig, syscall.SIGTERM)

	<-sig
	fmt.Println("signal caught - exiting")
	/*
		for {

		}
	*/ //var err error
	//aconf()
	//pconf()
	/*
			outboundVideoTrack, err = webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
				MimeType: "video/h264",
			}, "pion-rtsp", "pion-rtsp")
			if err != nil {
				panic(err)
			}

			go rtspConsumer()
			//go reportBuilder()
			startKVMClient()

		http.Handle("/", http.FileServer(http.Dir("./static")))
		http.HandleFunc("/doSignaling", doSignaling)

		fmt.Println("Open http://localhost:8080 to access this demo")
		panic(http.ListenAndServe(":8080", nil))
	*/
}
func parseConf() {
	if err := config.Parse(); err != nil {
		fmt.Println("cannot parse configuration file:", err)
		os.Exit(1)
	}
}

func main() {

	router := gin.Default()
	router.Use(Cors())
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./static")
	//server.InitDeviceHub()
	//startKVMClient()
	//	aconf()
	//	pconf()
	//parseConf()
	//kvm.StartKvmAgent()
	go TurnServer()

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	router.GET("/index", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"nav": "nav_home",
		})
	})
	router.Run("0.0.0.0:8080")

}
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method

		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type,AccessToken,X-CSRF-Token, Authorization, Token")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")

		//放行所有OPTIONS方法
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		// 处理请求
		c.Next()
	}
}

// The RTSP URL that will be streamed
//const rtspURL = "rtsp://170.93.143.139:1935/rtplive/0b01b57900060075004d823633235daa"
const kvmrtspURL = "rtsp://192.168.0.168/0"

// Connect to an RTSP URL and pull media.
// Convert H264 to Annex-B, then write to outboundVideoTrack which sends to all PeerConnections
func rtspConsumer() {

	annexbNALUStartCode := func() []byte { return []byte{0x00, 0x00, 0x00, 0x01} }

	for {
		session, err := rtsp.Dial(kvmrtspURL)
		if err != nil {
			panic(err)
		}
		session.RtpKeepAliveTimeout = 10 * time.Second

		codecs, err := session.Streams()
		if err != nil {
			panic(err)
		}
		for i, t := range codecs {
			log.Println("Stream", i, "is of type", t.Type().String())
		}
		if codecs[0].Type() != av.H264 {
			panic("RTSP feed must begin with a H264 codec")
		}
		if len(codecs) != 1 {
			log.Println("Ignoring all but the first stream.")
		}

		var previousTime time.Duration
		for {
			pkt, err := session.ReadPacket()
			if err != nil {
				break
			}

			if pkt.Idx != 0 {
				//audio or other stream, skip it
				continue
			}

			pkt.Data = pkt.Data[4:]

			// For every key-frame pre-pend the SPS and PPS
			if pkt.IsKeyFrame {
				pkt.Data = append(annexbNALUStartCode(), pkt.Data...)
				pkt.Data = append(codecs[0].(h264parser.CodecData).PPS(), pkt.Data...)
				pkt.Data = append(annexbNALUStartCode(), pkt.Data...)
				pkt.Data = append(codecs[0].(h264parser.CodecData).SPS(), pkt.Data...)
				pkt.Data = append(annexbNALUStartCode(), pkt.Data...)
			}

			bufferDuration := pkt.Time - previousTime
			previousTime = pkt.Time
			if err = rtspsrcmap[kvmrtspURL].WriteSample(media.Sample{Data: pkt.Data, Duration: bufferDuration}); err != nil && err != io.ErrClosedPipe {
				panic(err)
			}
		}

		if err = session.Close(); err != nil {
			log.Println("session Close error", err)
		}

		time.Sleep(5 * time.Second)
	}
}

func Notice(msg Message) {
	go doSignalingMqtt(msg)
}

type Session struct {
	Type     string `json:"type"`
	Msg      string `json:"msg"`
	Data     string `json:"data"`
	DeviceId string `json:"device_id"`
}

//"stun:stun.l.google.com:19302"
// MQTT Message Handler that accepts an Offer and returns an Answer
// adds outboundVideoTrack to PeerConnection
func doSignalingMqtt(msg Message) {
	//peerConnection, err := webrtc.NewPeerConnection(msg.Rtcconfig)
	fmt.Println("msg:", msg)
	fmt.Println("ICEServer:", msg.IceServer)
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:192.168.0.25:3478"},
				//URLs: msg.IceServer,
			},
			/*
				{
					URLs:           []string{"turn:192.168.0.25:3478"},
					Username:       "xxd",
					Credential:     "xxd",
					CredentialType: webrtc.ICECredentialTypePassword,
				},
			*/
		},

		SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		if connectionState == webrtc.ICEConnectionStateDisconnected {
			atomic.AddInt64(&peerConnectionCount, -1)
			if err := peerConnection.Close(); err != nil {
				panic(err)
			}
		} else if connectionState == webrtc.ICEConnectionStateConnected {
			atomic.AddInt64(&peerConnectionCount, 1)
		}
	})

	if _, err = peerConnection.AddTrack(rtspsrcmap[kvmrtspURL]); err != nil {
		panic(err)
	}

	//if msg.SSH
	peerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == "SSH" {
			//sshDataChannelHandler(dc)
		}
		if dc.Label() == "Control" {
			//controlDataChannelHandler(dc)
		}
		if dc.Label() == "Serial" {
			//serialDataChannelHandler(dc)
		}
		if dc.Label() == "HID" {
			HIDDataChannelHandler(dc)
		}
	})
	var offer webrtc.SessionDescription
	offer = msg.RtcSession
	fmt.Println("offer", offer)
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	gatherCompletePromise := webrtc.GatheringCompletePromise(peerConnection)

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	<-gatherCompletePromise
	/*
		response, err := json.Marshal(answer)
		if err != nil {
			panic(err)
		}
	*/
	req := &Session{}
	req.Type = "answer"
	req.DeviceId = "kvm1"
	req.Data = enc.Encode(answer)
	//data := signal.Encode(*peerConnection.LocalDescription())
	answermsg := PublishMsg{
		Topic: "answer",
		Msg:   req,
	}
	fmt.Println("answer", answermsg)
	SendMsg(answermsg) //response)
}

type MouseData struct {
	IsLeft bool `json:"isLeft"`
	IsDown bool `json:"isDown"`
	X      int  `json:"x"`
	Y      int  `json:"y"`
	Width  int  `json:"width"`
	Height int  `json:"height"`
}
type KeyData struct {
	KeyCode string `json:"keyCode"`
}
type HIDData struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Encode encodes the input in base64
// It can optionally zip the input before encoding
const (
	EVENT_MOUSE     = "MOUSE"
	EVENT_KEYDOWN   = "KEYDOWN"
	EVENT_MOUSEMOVE = "MOUSEMOVE"
)

func HIDDataChannelHandler(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {

	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		var hid HIDData
		err := json.Unmarshal(msg.Data, &hid)
		if err != nil {
			return
		}
		switch hid.Type {
		case EVENT_MOUSE:
			fmt.Println("mouse", hid.Data)
			//HIDserialHandler()
		case EVENT_MOUSEMOVE:
			fmt.Println("mousemove", hid.Data)
		case EVENT_KEYDOWN:
			fmt.Println("key", hid.Data)

		}
		fmt.Println("HID", hid)
	})
	dc.OnClose(func() {
		fmt.Printf("Close Control socket")
	})
}

//kcom3 HID 模拟模块接口
//鼠标数据 绝对坐标
//字节: byte1 byet2 byte3 byte01 byte02 byte03  byte04 byte05   byte06
//head: 0x57  0xAB  0x04           低字节在前，高字节在后
//鼠标                    按键    X轴绝对位移值   Y轴绝对位移值     滚轮(0x01-0x07 表示向上滚齿数 0x81-0xFF表)
//0x01 左键按下 0x02 右键按下 0x04 中键按下

func SendHIDtoDevice(data []byte) {

}
