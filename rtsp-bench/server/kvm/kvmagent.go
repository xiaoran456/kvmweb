package kvm

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/format/rtsp"
	webrtcdeep "github.com/deepch/vdk/format/webrtcv3"
	"github.com/pion/rtsp-bench/server/config"
	enc "github.com/pion/rtsp-bench/server/signal"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/shirou/gopsutil/cpu"
)

var (
	//vers                *bool
	//help                *bool
	//conf                *string
	outboundVideoTrack  *webrtc.TrackLocalStaticSample
	peerConnectionCount int64
	CurrentKVMRTSP      RTSPInfo
	rtspsrcch           chan RTSPInfo
	rtspsrcmap          map[string]*VIDEO_SRC
	hidkeymap           map[byte]HidKeyCode
	//Version             = "v0.1.0"
	//outboundVideoTrack  *webrtc.TrackLocalStaticSample
	//peerConnectionCount int64
)

const (
	DEVICE_IP     = 100
	DEVICE_ONVIF  = 200
	DEVICE_SNMP   = 300
	DEVICE_MODBUS = 400
	DEVICE_BACNET = 500
	DEVICE_CAN    = 600
	DEVICE_UPCA   = 700
	CMDMSG_OFFER  = "offer"
	CMDMSG_DISC   = "discovery"
	CMDMSG_WAKE   = "wake"
	CMDMSG_UPDATE = "update"
	CMDMSG_MR2    = "mr2"
	//CMDMSG_OFF    = "off"

)

//https://github.com/txthinking/mr2/blob/master/README_ZH.md
//利用mr2内网映射软件做网络NAT 代理
//暴露本地局域网192.168.0.25的 ssh访问地址
//$ mr2 client -s 1.2.3.4:9999 -p password -P 8888 -c 192.168.0.25:22
//ssh -oPort=8888 yourlocaluser@1.2.3.4 按此进行内网192.168.0.25的ssh服务进行连接访问
//$ mr2 client -s 1.2.3.4:9999 -p password -P 8888 --clientDirectory /path/to/www --clientPort 8080
//现在访问 1.2.3.4:8888 就等于 127.0.0.1:8080, web root 是 /path/to/www
type Mr2Msg struct {
	ServerAddr string //运行mr2服务器的地址  1.2.3.4:9000
	Password   string //连接的密码
	ExposePort int    //映射的端口 8888
	ExposeAddr string //需要映射的局域网内的地址和端口 192.168.0.25:8888
}
type MAC struct {
	Host string
	Mac  string
}
type ResponseMsg struct {
	Cmdstr string
	Status int
	Err    string
	Sid    string
}
type versionUpdate struct {
	ForceUpdate int    `json:"forceupdate"` // 是否需要强制更新 0是否 1是需要
	Version     string `json:"version"`     // 版本号字符串
	DownLoadUrl string `json:"downloadurl"` // 下载地址
	VersionDesc string `json:"versiondesc"` // 更新描述
}
type DiscoveryCmd struct {
	DevType int `json:"DevType"`
}
type Session struct {
	Type     string `json:"type"`
	Msg      string `json:"msg"`
	Data     string `json:"data"`
	DeviceId string `json:"device_id"`
}
type PublishMsg struct {
	Topic string
	Msg   interface{}
}

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

type VIDEO_SRC struct {
	Name           string
	RtspServerAddr string
	Resolution     string
	Track          *webrtc.TrackLocalStaticSample
	BInUse         bool
}
type RTSPInfo struct {
	Suuid string
	URL   string
}
type MouseData struct {
	IsLeft   int `json:"isLeft"`
	IsMiddle int `json:"isMiddle"`
	IsRight  int `json:"isRight"`
	IsDown   int `json:"isDown"`
	X        int `json:"x"`
	Y        int `json:"y"`
	Width    int `json:"width"`
	Height   int `json:"height"`
}
type KeyData struct {
	FuncKey byte `json:"funcKey"`
	KeyCode byte `json:"keyCode"`
}
type HIDData struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}
type HidKeyCode struct {
	KeyCode byte
	Shift   bool
}

// Encode encodes the input in base64
// It can optionally zip the input before encoding
const (
	EVENT_MOUSE     = "MOUSE"
	EVENT_MOUSEUP   = "MOUSEUP"
	EVENT_MOUSEDOWN = "MOUSEDOWN"
	EVENT_KEYDOWN   = "KEYDOWN"
	EVENT_MOUSEMOVE = "MOUSEMOVE"
)

//https://blog.csdn.net/weixin_33905756/article/details/94002203
//https://blog.csdn.net/weixin_33905756/article/details/94002203
func init() {
	/*
		#define Keyboard_a                4   // Keyboard a and A
		#define Keyboard_b                5   // Keyboard b and B
		#define Keyboard_c                6   // Keyboard c and C
		#define Keyboard_d                7   // Keyboard d and D
		#define Keyboard_e                8   // Keyboard e and E
		#define Keyboard_f                9   // Keyboard f and F
		#define Keyboard_g                10  // Keyboard g and G
		#define Keyboard_h                11  // Keyboard h and H
		#define Keyboard_i                12  // Keyboard i and I
		#define Keyboard_j                13  // Keyboard j and J
		#define Keyboard_k                14  // Keyboard k and K
		#define Keyboard_l                15  // Keyboard l and L
		#define Keyboard_m                16  // Keyboard m and M
		#define Keyboard_n                17  // Keyboard n and N
		#define Keyboard_o                18  // Keyboard o and O
		#define Keyboard_p                19  // Keyboard p and P
		#define Keyboard_q                20  // Keyboard q and Q
		#define Keyboard_r                21  // Keyboard r and R
		#define Keyboard_s                22  // Keyboard s and S
		#define Keyboard_t                23  // Keyboard t and T
		#define Keyboard_u                24  // Keyboard u and U
		#define Keyboard_v                25  // Keyboard v and V
		#define Keyboard_w                26  // Keyboard w and W
		#define Keyboard_x                27  // Keyboard x and X
		#define Keyboard_y                28  // Keyboard y and Y
		#define Keyboard_z                29  // Keyboard z and Z
		#define Keyboard_1                30  // Keyboard 1 and !
		#define Keyboard_2                31  // Keyboard 2 and @
		#define Keyboard_3                32  // Keyboard 3 and #
		#define Keyboard_4                33  // Keyboard 4 and $
		#define Keyboard_5                34  // Keyboard 5 and %
		#define Keyboard_6                35  // Keyboard 6 and ^
		#define Keyboard_7                36  // Keyboard 7 and &
		#define Keyboard_8                37  // Keyboard 8 and *
		#define Keyboard_9                38  // Keyboard 9 and (
		#define Keyboard_0                39  // Keyboard 0 and )
		#define Keyboard_ENTER            40  // Keyboard ENTER
		#define Keyboard_ESCAPE           41  // Keyboard ESCAPE
		#define Keyboard_Backspace        42  // Keyboard Backspace
		#define Keyboard_Tab              43  // Keyboard Tab
		#define Keyboard_KongGe           44  // Keyboard Spacebar
		#define Keyboard_JianHao          45  // Keyboard - and _(underscore)
		#define Keyboard_DengHao          46  // Keyboard = and +
		#define Keyboard_ZuoZhongKuoHao   47  // Keyboard [ and {
		#define Keyboard_YouZhongKuoHao   48  // Keyboard ] and }
		#define Keyboard_FanXieGang       49  // Keyboard \ and |
		#define Keyboard_FenHao           51  // Keyboard ; and :
		#define Keyboard_DanYinHao        52  // Keyboard ‘ and “
		#define Keyboard_BoLangXian       53  // Keyboard `(Grave Accent) and ~(Tilde)
		#define Keyboard_Douhao           54  // Keyboard, and <
		#define Keyboard_JuHao            55  // Keyboard . and >
		#define Keyboard_XieGang_WenHao   56  // Keyboard / and ?
		#define Keyboard_CapsLock         57  // Keyboard Caps Lock
		#define Keyboard_F1               58  // Keyboard F1
		#define Keyboard_F2               59  // Keyboard F2
		#define Keyboard_F3               60  // Keyboard F3
		#define Keyboard_F4               61  // Keyboard F4
		#define Keyboard_F5               62  // Keyboard F5
		#define Keyboard_F6               63  // Keyboard F6
		#define Keyboard_F7               64  // Keyboard F7
		#define Keyboard_F8               65  // Keyboard F8
		#define Keyboard_F9               66  // Keyboard F9
		#define Keyboard_F10              67  // Keyboard F10
		#define Keyboard_F11              68  // Keyboard F11
		#define Keyboard_F12              69  // Keyboard F12
		#define Keyboard_PrintScreen      70  // Keyboard PrintScreen
		#define Keyboard_ScrollLock       71  // Keyboard Scroll Lock
		#define Keyboard_Pause            72  // Keyboard Pause
		#define Keyboard_Insert           73  // Keyboard Insert
		#define Keyboard_Home             74  // Keyboard Home
		#define Keyboard_PageUp           75  // Keyboard PageUp
		#define Keyboard_Delete           76  // Keyboard Delete
		#define Keyboard_End              77  // Keyboard End
		#define Keyboard_PageDown         78  // Keyboard PageDown
		#define Keyboard_RightArrow       79  // Keyboard RightArrow
		#define Keyboard_LeftArrow        80  // Keyboard LeftArrow
		#define Keyboard_DownArrow        81  // Keyboard DownArrow
		#define Keyboard_UpArrow          82  // Keyboard UpArrow
		#define Keypad_NumLock            83  // Keypad Num Lock and Clear
		#define Keypad_ChuHao             84  // Keypad /
		#define Keypad_ChengHao           85  // Keypad *
		#define Keypad_JianHao            86  // Keypad -
		#define Keypad_JiaHao             87  // Keypad +
		#define Keypad_ENTER              88  // Keypad ENTER
		#define Keypad_1_and_End          89  // Keypad 1 and End
		#define Keypad_2_and_DownArrow    90  // Keypad 2 and Down Arrow
		#define Keypad_3_and_PageDn       91  // Keypad 3 and PageDn
		#define Keypad_4_and_LeftArrow    92  // Keypad 4 and Left Arrow
		#define Keypad_5                  93  // Keypad 5
		#define Keypad_6_and_RightArrow   94  // Keypad 6 and Right Arrow
		#define Keypad_7_and_Home         95  // Keypad 7 and Home
		#define Keypad_8_and_UpArrow      96  // Keypad 8 and Up Arrow
		#define Keypad_9_and_PageUp       97  // Keypad 9 and PageUp
		#define Keypad_0_and_Insert       98  // Keypad 0 and Insert
		#define Keypad_Dian_and_Delete    99  // Keypad . and Delete
		#define Keyboard_Application      101 // Keyboard Application
		#define Keyboard_LeftControl      224
		#define Keyboard_LeftShift        225
		#define Keyboard_LeftAlt          226
		#define Keyboard_LeftWindows      227
		#define Keyboard_RightControl     228
		#define Keyboard_RightShift       229
		#define Keyboard_RightAlt         230
		#define Keyboard_RightWindows     231

	*/
	hidkeymap = make(map[byte]HidKeyCode)
	//hidkeymap['POSTFail'] = 0x02
	//hidkeymap['ErrorUndefined'] = 0x03
	hidkeymap[65] = HidKeyCode{0x04, false} //a
	//hidkeymap['A'] = HidKeyCode{0x04, true} //b
	hidkeymap[66] = HidKeyCode{0x05, false}
	//hidkeymap['B'] = HidKeyCode{0x05, true}
	hidkeymap[67] = HidKeyCode{0x06, false}
	//hidkeymap['C'] = HidKeyCode{0x06, true}
	hidkeymap[68] = HidKeyCode{0x07, false}
	//hidkeymap['D'] = HidKeyCode{0x07, true}
	hidkeymap[69] = HidKeyCode{0x08, false}
	//hidkeymap['E'] = HidKeyCode{0x08, true}
	hidkeymap[70] = HidKeyCode{0x09, false}
	//hidkeymap['F'] = HidKeyCode{0x09, true}
	hidkeymap[71] = HidKeyCode{0x0A, false}
	//hidkeymap['G'] = HidKeyCode{0x0A, true}
	hidkeymap[72] = HidKeyCode{0x0B, false}
	//hidkeymap['H'] = HidKeyCode{0x0B, true}
	hidkeymap[73] = HidKeyCode{0x0C, false}
	//hidkeymap['I'] = HidKeyCode{0x0C, true}
	hidkeymap[74] = HidKeyCode{0x0D, false}
	//hidkeymap['J'] = HidKeyCode{0x0D, true}
	hidkeymap[75] = HidKeyCode{0x0E, false}
	//hidkeymap['K'] = HidKeyCode{0x0E, true}
	hidkeymap[76] = HidKeyCode{0x0F, false}
	//hidkeymap['L'] = HidKeyCode{0x0F, true}
	hidkeymap[77] = HidKeyCode{0x10, false}
	//hidkeymap['M'] = HidKeyCode{0x10, true}
	hidkeymap[78] = HidKeyCode{0x11, false}
	//hidkeymap['N'] = HidKeyCode{0x11, true}
	hidkeymap[79] = HidKeyCode{0x12, false}
	//hidkeymap['O'] = HidKeyCode{0x12, true}
	hidkeymap[80] = HidKeyCode{0x13, false}
	//hidkeymap['P'] = HidKeyCode{0x13, true}
	hidkeymap[81] = HidKeyCode{0x14, false}
	//hidkeymap['Q'] = HidKeyCode{0x14, true}
	hidkeymap[82] = HidKeyCode{0x15, false}
	//hidkeymap['R'] = HidKeyCode{0x15, true}
	hidkeymap[83] = HidKeyCode{0x16, false}
	//hidkeymap['S'] = HidKeyCode{0x16, true}
	hidkeymap[84] = HidKeyCode{0x17, false}
	//hidkeymap['T'] = HidKeyCode{0x17, true}
	hidkeymap[85] = HidKeyCode{0x18, false}
	//hidkeymap['U'] = HidKeyCode{0x18, true}
	hidkeymap[86] = HidKeyCode{0x19, false}
	//hidkeymap['V'] = HidKeyCode{0x19, true}
	hidkeymap[87] = HidKeyCode{0x1A, false}
	//hidkeymap['W'] = HidKeyCode{0x1A, true}
	hidkeymap[88] = HidKeyCode{0x1B, false}
	//hidkeymap['X'] = HidKeyCode{0x1B, true}
	hidkeymap[89] = HidKeyCode{0x1C, false}
	//hidkeymap['Y'] = HidKeyCode{0x1C, true}
	hidkeymap[90] = HidKeyCode{0x1D, false}
	//hidkeymap['Z'] = HidKeyCode{0x1D, true}
	hidkeymap['1'] = HidKeyCode{0x1E, false}
	//hidkeymap['!'] = HidKeyCode{0x1E, true}
	hidkeymap['2'] = HidKeyCode{0x1F, false}
	//hidkeymap['@'] = HidKeyCode{0x1F, true}
	hidkeymap['3'] = HidKeyCode{0x20, false}
	//hidkeymap['#'] = HidKeyCode{0x20, true}
	hidkeymap['4'] = HidKeyCode{0x21, false}
	//hidkeymap['#'] = HidKeyCode{0x21, true}
	hidkeymap['5'] = HidKeyCode{0x22, false}
	//hidkeymap['%'] = HidKeyCode{0x22, true}
	hidkeymap['6'] = HidKeyCode{0x23, false}
	//hidkeymap['^'] = HidKeyCode{0x23, true}
	hidkeymap['7'] = HidKeyCode{0x24, false}
	//hidkeymap['&'] = HidKeyCode{0x24, true}
	hidkeymap['8'] = HidKeyCode{0x25, false}
	//hidkeymap['*'] = HidKeyCode{0x25, true}
	hidkeymap['9'] = HidKeyCode{0x26, false}
	//hidkeymap['('] = HidKeyCode{0x26, true}
	hidkeymap['0'] = HidKeyCode{0x27, false}
	//hidkeymap[')'] = HidKeyCode{0x27, true}

	hidkeymap['\r'] = HidKeyCode{0x28, false}
	hidkeymap[27] = HidKeyCode{0x29, false} //"ESC"
	hidkeymap[8] = HidKeyCode{0x2A, false}  //'DELETE (Backspace)'
	hidkeymap[9] = HidKeyCode{0x2B, false}  //'Tab'
	hidkeymap[' '] = HidKeyCode{0x2C, false}
	hidkeymap[189] = HidKeyCode{0x2D, false}
	//	hidkeymap['(underscore)'] = HidKeyCode{0x2D,false}
	hidkeymap[187] = HidKeyCode{0x2E, false}
	//hidkeymap['+'] = HidKeyCode{0x2E, true}
	hidkeymap[219] = HidKeyCode{0x2F, false}
	//hidkeymap['{'] = HidKeyCode{0x2F, true}
	hidkeymap[221] = HidKeyCode{0x30, false}
	//hidkeymap['}'] = HidKeyCode{0x30, true}
	hidkeymap[220] = HidKeyCode{0x31, false} //'\'
	//hidkeymap['|'] = HidKeyCode{0x31, true}
	//hidkeymap['Non-US #'] = HidKeyCode{0x32,false}
	hidkeymap[192] = HidKeyCode{0x32, false}
	hidkeymap[186] = HidKeyCode{0x33, false}
	//hidkeymap[':'] = HidKeyCode{0x33, true}
	hidkeymap[222] = HidKeyCode{0x34, false}
	//hidkeymap[222] = HidKeyCode{0x34, true} //"\""
	//	hidkeymap['Grave Accent'] = HidKeyCode{0x35,false}
	//	hidkeymap['Tilde'] = HidKeyCode{0x35,false}
	//	hidkeymap['Keyboard,'] = HidKeyCode{0x36,false}
	hidkeymap[188] = HidKeyCode{0x36, false}
	//hidkeymap['<'] = HidKeyCode{0x36, true}
	hidkeymap[190] = HidKeyCode{0x37, false}
	//hidkeymap['>'] = HidKeyCode{0x37, true}
	hidkeymap[191] = HidKeyCode{0x38, false}
	//hidkeymap['?'] = HidKeyCode{0x38, true}
	hidkeymap[20] = HidKeyCode{0x39, false}  //backspace
	hidkeymap[39] = HidKeyCode{0x4F, false}  //right arrow
	hidkeymap[37] = HidKeyCode{0x50, false}  //left arrow
	hidkeymap[40] = HidKeyCode{0x51, false}  //down arrow
	hidkeymap[38] = HidKeyCode{0x52, false}  //up arrow
	hidkeymap[112] = HidKeyCode{0x3A, false} //f1
	hidkeymap[113] = HidKeyCode{0x3B, false} //f2
	hidkeymap[114] = HidKeyCode{0x3C, false}
	hidkeymap[115] = HidKeyCode{0x3D, false}
	hidkeymap[116] = HidKeyCode{0x3E, false}
	hidkeymap[117] = HidKeyCode{0x3F, false}
	hidkeymap[118] = HidKeyCode{0x40, false}
	hidkeymap[119] = HidKeyCode{0x41, false}
	hidkeymap[120] = HidKeyCode{0x42, false}
	hidkeymap[121] = HidKeyCode{0x43, false}
	hidkeymap[122] = HidKeyCode{0x44, false}
	hidkeymap[123] = HidKeyCode{0x45, false} //f12
	hidkeymap[19] = HidKeyCode{0x48, false}  //pause
	hidkeymap[46] = HidKeyCode{0x49, false}  //Insert
	/*
		hidkeymap['F1'] = HidKeyCode{0x3A
		hidkeymap['F2'] = HidKeyCode{0x3B
		hidkeymap['F3'] = HidKeyCode{0x3C
		hidkeymap['F4'] = HidKeyCode{0x3D
		hidkeymap['F5'] = HidKeyCode{0x3E
		hidkeymap['F6'] = HidKeyCode{0x3F
		hidkeymap['F7'] = HidKeyCode{0x40
		hidkeymap['F8'] = HidKeyCode{0x41
		hidkeymap['F9'] = HidKeyCode{0x42
		hidkeymap['F10'] = HidKeyCode{0x43
		hidkeymap['F11'] = HidKeyCode{0x44
		hidkeymap['F12'] = HidKeyCode{0x45

		hidkeymap['PrintScreen'] = HidKeyCode{0x46
		hidkeymap['Scroll Lock'] = HidKeyCode{0x47
		hidkeymap['Pause'] = HidKeyCode{0x48
		hidkeymap['Insert'] = HidKeyCode{0x49
		hidkeymap['Home'] = HidKeyCode{0x4A
		hidkeymap['PageUp'] = HidKeyCode{0x4B
		hidkeymap['Delete Forward'] = HidKeyCode{0x4C
		hidkeymap['End'] = HidKeyCode{0x4D
		hidkeymap['PageDown'] = HidKeyCode{0x4E
		hidkeymap['RightArrow'] = HidKeyCode{x4F
		hidkeymap['LeftArrow'] = HidKeyCode{0x50
		hidkeymap['DownArrow'] = HidKeyCode{0x51
		hidkeymap['UpArrow'] = HidKeyCode{0x52
		hidkeymap['Num Lock'] = HidKeyCode{0x53
		hidkeymap['Clear'] = HidKeyCode{0x53
		hidkeymap['/'] = HidKeyCode{0x54
		hidkeymap['*'] = HidKeyCode{0x55
		hidkeymap['-'] = HidKeyCode{0x56
		hidkeymap['+'] = HidKeyCode{0x57
		hidkeymap['ENTER'] = HidKeyCode{0x58
		hidkeymap['1'] = HidKeyCode{0x59
		hidkeymap['Down Arrow'] = HidKeyCode{0x5A
		hidkeymap['2'] = HidKeyCode{0x5A
		hidkeymap['PageDn'] = HidKeyCode{0x5B
		hidkeymap['3'] = HidKeyCode{0x5B
		hidkeymap['Left Arrow'] = HidKeyCode{0x5C
		hidkeymap['4'] = HidKeyCode{0x5C
		hidkeymap['5'] = HidKeyCode{0x5D
		hidkeymap['Right Arrow'] = HidKeyCode{0x5E
		hidkeymap['6'] = HidKeyCode{0x5E
		hidkeymap['Home'] = HidKeyCode{0x5F
		hidkeymap['7'] = HidKeyCode{0x5F
		hidkeymap['Up Arrow'] = HidKeyCode{0x60
		hidkeymap['8'] = HidKeyCode{0x60
		hidkeymap['PageUp'] = HidKeyCode{0x61
		hidkeymap['9'] = HidKeyCode{0x61
		hidkeymap['Insert'] = HidKeyCode{0x62
		hidkeymap['0'] = HidKeyCode{0x62
		hidkeymap['.'] = HidKeyCode{0x63
		hidkeymap['Delete'] = HidKeyCode{0x63
		hidkeymap['Non-US \\'] = HidKeyCode{0x64
		hidkeymap['|'] = HidKeyCode{0x64
		hidkeymap['Application'] = HidKeyCode{0x65
		hidkeymap['Power'] = HidKeyCode{0x66
		hidkeymap['='] = HidKeyCode{0x67
		hidkeymap['F13'] = HidKeyCode{0x68
		hidkeymap['F14'] = HidKeyCode{0x69
		hidkeymap['F15'] = HidKeyCode{0x6A
		hidkeymap['F16'] = HidKeyCode{0x6B
		hidkeymap['F17'] = HidKeyCode{0x6C
		hidkeymap['F18'] = HidKeyCode{0x6D
		hidkeymap['F19'] = HidKeyCode{0x6E
		hidkeymap['F21'] = HidKeyCode{0x6F
		hidkeymap['F22'] = HidKeyCode{0x70
		hidkeymap['F23'] = HidKeyCode{0x71
		hidkeymap['F24'] = HidKeyCode{0x72
		hidkeymap['F18'] = HidKeyCode{0x73
		hidkeymap['Execute'] = HidKeyCode{0x74
		hidkeymap['Help'] = HidKeyCode{0x75
		hidkeymap['Menu'] = HidKeyCode{0x76
		hidkeymap['Select'] = HidKeyCode{0x77
		hidkeymap['Stop'] = HidKeyCode{0x78
		hidkeymap['Again'] = HidKeyCode{0x79
		hidkeymap['Undo'] = HidKeyCode{0x7A
		hidkeymap['Cut'] = HidKeyCode{0x7B
		hidkeymap['Copy'] = HidKeyCode{0x7C
		hidkeymap['Paste'] = HidKeyCode{0x7D
		hidkeymap['Find'] = HidKeyCode{0x7E
		hidkeymap['Mute'] = HidKeyCode{0x7F
		hidkeymap['Volume Up'] = HidKeyCode{0x80
		hidkeymap['Volume Down'] = HidKeyCode{0x81
		hidkeymap['Locking Caps Lock'] = HidKeyCode{0x82
		hidkeymap['Locking Num Lock'] = HidKeyCode{0x83
		hidkeymap['Locking Scroll Lock'] = HidKeyCode{0x84
		hidkeymap['Comma'] = HidKeyCode{0x85
		hidkeymap['Equal Sign'] = HidKeyCode{0x86
		hidkeymap['International1'] = HidKeyCode{0x87
		hidkeymap['International2'] = HidKeyCode{0x88
		hidkeymap['International3'] = HidKeyCode{0x89
		hidkeymap['International4'] = HidKeyCode{0x8A
		hidkeymap['International5'] = HidKeyCode{0x8B
		hidkeymap['International6'] = HidKeyCode{0x8C
		hidkeymap['International7'] = HidKeyCode{0x8D
		hidkeymap['International8'] = HidKeyCode{0x8E
		hidkeymap['International9'] = HidKeyCode{0x8F
		hidkeymap['LANG1'] = HidKeyCode{0x90
		hidkeymap['LANG2'] = HidKeyCode{0x91
		hidkeymap['LANG3'] = HidKeyCode{0x92
		hidkeymap['LANG4'] = HidKeyCode{0x93
		hidkeymap['LANG5'] = HidKeyCode{0x94
		hidkeymap['LANG6'] = HidKeyCode{0x95
		hidkeymap['LANG7'] = HidKeyCode{0x96
		hidkeymap['LANG8'] = HidKeyCode{0x97
		hidkeymap['LANG9'] = HidKeyCode{0x98
		hidkeymap['Alternate Erase'] = HidKeyCode{0x99
		hidkeymap['SysReq/Attention'] = HidKeyCode{0x9A
		hidkeymap['Cancel'] = HidKeyCode{0x9B
		hidkeymap['Clear'] = HidKeyCode{0x9C
		hidkeymap['Prior'] = HidKeyCode{0x9D
		hidkeymap['Return'] = HidKeyCode{0x9E
		hidkeymap['Separator'] = HidKeyCode{0x9F
		hidkeymap['Out'] = HidKeyCode{0xA0
		hidkeymap['Oper'] = HidKeyCode{0xA1
		hidkeymap['Clear/Again'] = HidKeyCode{0xA2
		hidkeymap['CrSel/Props'] = HidKeyCode{0xA3
		hidkeymap['ExSel'] = HidKeyCode{0xA4
		hidkeymap['LeftControl'] = HidKeyCode{0xE0
		hidkeymap['LeftShift'] = HidKeyCode{0xE1
		hidkeymap['LeftAlt'] = HidKeyCode{0xE2
		hidkeymap['Left GUI'] = HidKeyCode{0xE3
		hidkeymap['RightControl'] = HidKeyCode{0xE4
		hidkeymap['RightShift'] = HidKeyCode{0xE5
		hidkeymap['RightAlt'] = HidKeyCode{0xE6
		hidkeymap['Right GUI'] = HidKeyCode{0xE7
	*/
}

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

/*
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
*/
/*
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
*/
func startKVMClient() {
	//uiTest()
	go StartMqtt()
	//go screen.StartScreentShot()
}

func StartKvmAgent() {
	var err error
	//aconf()
	//pconf()
	rtspsrcmap = make(map[string]*VIDEO_SRC)
	rtspsrcch = make(chan RTSPInfo)
	outboundVideoTrack, err = webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType: "video/h264",
	}, "pion-rtsp", "pion-rtsp")
	if err != nil {
		panic(err)
	}
	kvm1 := VIDEO_SRC{
		Name:           "KVMStream1",
		RtspServerAddr: config.Config.KVM.RTSP1ServerAddr,
		Resolution:     "1920*1080@30",
		Track:          outboundVideoTrack,
	}
	rtspsrcmap["KVMStream1"] = &kvm1
	kvm2 := VIDEO_SRC{
		Name:           "KVMStream2",
		RtspServerAddr: config.Config.KVM.RTSP2ServerAddr,
		Resolution:     "1280*720@30",
		Track:          outboundVideoTrack,
	}
	rtspsrcmap["KVMStream2"] = &kvm2
	kvm3 := VIDEO_SRC{
		Name:           "KVMStream3",
		RtspServerAddr: config.Config.KVM.RTSP3ServerAddr,
		Resolution:     "640*360@30",
		Track:          outboundVideoTrack,
	}
	rtspsrcmap["KVMStream3"] = &kvm3
	kvm4 := VIDEO_SRC{
		Name:           "KVMStream4",
		RtspServerAddr: config.Config.KVM.RTSP4ServerAddr,
		Resolution:     "640*360@30",
		Track:          outboundVideoTrack,
	}
	rtspsrcmap["KVMStream4"] = &kvm4
	go rtspConsumer()
	go HIDSerailTask()
	//HIDserialOpen()
	//go reportBuilder()
	startKVMClient()
	/*
		http.Handle("/", http.FileServer(http.Dir("./static")))
		http.HandleFunc("/doSignaling", doSignaling)

		fmt.Println("Open http://localhost:8080 to access this demo")
		panic(http.ListenAndServe(":8080", nil))
	*/
}
func SwitchSrcIndex(srcindex RTSPInfo) {
	for _, v := range rtspsrcmap {
		v.BInUse = false
	}
	//time.Sleep(time.Millisecond * 5)
	index := rtspsrcmap[srcindex.Suuid]
	if index != nil {
		//index.BInUse = true
		rtspsrcch <- srcindex
		fmt.Println("Switch to:", srcindex.Suuid, srcindex.URL)
	} else {
		scrindex := VIDEO_SRC{
			Name:           srcindex.Suuid,
			RtspServerAddr: srcindex.URL,
			Resolution:     "1920*1080@30",
			Track:          outboundVideoTrack,
			BInUse:         true,
		}
		rtspsrcmap[srcindex.Suuid] = &scrindex
		rtspsrcch <- srcindex
		fmt.Println("Add New Src&Switch to:", srcindex.Suuid, srcindex.URL)
	}
}

// The RTSP URL that will be streamed
//const rtspURL = "rtsp://170.93.143.139:1935/rtplive/0b01b57900060075004d823633235daa"
//const KVMrtspURL = "rtsp://127.0.0.1/0"

// Connect to an RTSP URL and pull media.
// Convert H264 to Annex-B, then write to outboundVideoTrack which sends to all PeerConnections
func rtspConsumer() {
	annexbNALUStartCode := func() []byte { return []byte{0x00, 0x00, 0x00, 0x01} }

	for {
		current := <-rtspsrcch

		rtspsrcmap[current.Suuid].BInUse = true
		KVMrtsp := rtspsrcmap[current.Suuid]
		//psrcIndex:=
		//KVMrtsp := rtspsrcmap["KVMStream3"]
		//if
		if KVMrtsp == nil {
			time.Sleep(time.Second * 2)
			continue
		}

		session, err := rtsp.Dial(KVMrtsp.RtspServerAddr)
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
			if KVMrtsp.BInUse != true {
				break
			}
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
			if err = KVMrtsp.Track.WriteSample(media.Sample{Data: pkt.Data, Duration: bufferDuration}); err != nil && err != io.ErrClosedPipe {
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
	//go RTSP2StreamWebRTC(msg)
}

//"stun:stun.l.google.com:19302"
// MQTT Message Handler that accepts an Offer and returns an Answer
// adds outboundVideoTrack to PeerConnection
func doSignalingMqtt(msg Message) {
	fmt.Println("msg", msg)

	CurrentKVMRTSP.Suuid = msg.Suuid
	CurrentKVMRTSP.URL = msg.VideoRtspServerAddr
	if rtspsrcmap[msg.Suuid] == nil {
		fmt.Println(msg.Suuid, "is not exist")
		return
	}
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				//URLs: []string{"stun:192.168.0.25:3478"},
				URLs: msg.IceServer,
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

	if msg.Video {
		//rtspsrcch <- CurrentKVMRTSP
		SwitchSrcIndex(CurrentKVMRTSP)
		if _, err = peerConnection.AddTrack(rtspsrcmap[CurrentKVMRTSP.Suuid].Track); err != nil {
			panic(err)
		}
	}
	//if msg.SSH
	peerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == "SSH" {
			sshDataChannelHandler(dc)
		}
		if dc.Label() == "Control" {
			controlDataChannelHandler(dc)
		}
		if dc.Label() == "Serial" {
			serialDataChannelHandler(dc)
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
	req.Data = enc.Encode(*peerConnection.LocalDescription())
	//data := signal.Encode(*peerConnection.LocalDescription())
	answermsg := PublishMsg{
		Topic: "answer",
		Msg:   req,
	}
	fmt.Println("answer", answermsg)
	SendMsg(answermsg) //response)
}

//"stun:stun.l.google.com:19302"
// MQTT Message Handler that accepts an Offer and returns an Answer
// adds outboundVideoTrack to PeerConnection
func doSignalingMqtt_1(msg Message) {
	//peerConnection, err := webrtc.NewPeerConnection(msg.Rtcconfig)
	fmt.Println("msg", msg)
	CurrentKVMRTSP.Suuid = msg.Suuid
	CurrentKVMRTSP.URL = msg.VideoRtspServerAddr
	if rtspsrcmap[msg.Suuid] == nil {
		fmt.Println(msg.Suuid, "is not exist")
		return
		//AddRtsptoMap(VideoRtspServerAddr)
		/*
			respmsg := PublishMsg{
				Topic: "error",
				Msg:   req,
			}
			fmt.Println("answer", answermsg)
			SendMsg(answermsg) //response)
		*/
	}
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				//URLs: []string{"stun:192.168.0.25:3478"},
				URLs: msg.IceServer,
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
	if msg.Video {
		SwitchSrcIndex(CurrentKVMRTSP)
		if _, err = peerConnection.AddTrack(rtspsrcmap[CurrentKVMRTSP.Suuid].Track); err != nil {
			panic(err)
		}
	}

	//if msg.SSH
	peerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == "SSH" {
			sshDataChannelHandler(dc)
		}
		if dc.Label() == "Control" {
			controlDataChannelHandler(dc)
		}
		if dc.Label() == "Serial" {
			serialDataChannelHandler(dc)
		}
		if dc.Label() == "HID" {
			HIDDataChannelHandler(dc)
		}
	})

	var offer webrtc.SessionDescription
	offer = msg.RtcSession
	fmt.Println("ofer", offer)
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
	/*
		response, err := json.Marshal(answer)
		if err != nil {
			panic(err)
		}
	*/
	/*
		answermsg := PublishMsg{
			Topic: "answer",
			Msg:   *peerConnection.LocalDescription(),
		}
		SendMsg(answermsg) //response)
	*/
}
func controlDataChannelHandler(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		err := dc.SendText("please input command")
		if err != nil {
			fmt.Println("write data error:", err)
			dc.Close()
		}
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		result := controlHandler(msg.Data)
		dc.SendText(result)
	})
	dc.OnClose(func() {
		fmt.Printf("Close Control socket")
	})
}
func serialDataChannelHandler(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		err := dc.SendText("please input command")
		if err != nil {
			fmt.Println("write data error:", err)
			dc.Close()
		}
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		result := serialHandler(msg.Data)
		dc.SendText(result)
	})
	dc.OnClose(func() {
		fmt.Printf("Close Control socket")
	})
}
func sshDataChannelHandler(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		for {
			var user string
			var password string
			var addr string
			rtcin := make(chan string)
			step := make(chan string)

			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				addr = string(msg.Data)
				fmt.Println(addr)
				dc.OnMessage(func(msg webrtc.DataChannelMessage) {
					user = string(msg.Data)
					fmt.Println(user)
					dc.OnMessage(func(msg webrtc.DataChannelMessage) {
						password = string(msg.Data)
						fmt.Println(password)
						step <- ""
					})
				})
			})

			<-step
			if strings.Contains(addr, ":") != true {
				addr = fmt.Sprintf("%s:%d", config.Config.KVM.SSHHost, config.Config.KVM.SSHPort)
			}
			sshSession, err := initSSH(user, password, addr, dc, rtcin)
			if err != nil {
				dc.SendText(err.Error())
				continue
			}
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				msg_ := string(msg.Data)

				if len(msg_) >= 10 {
					ss := strings.Fields(msg_)
					if ss[0] == "resize" {
						cols, _ := strconv.Atoi(ss[1])
						rows, _ := strconv.Atoi(ss[2])
						sshSession.WindowChange(cols, rows)
						fmt.Println(msg_)
						return
					}
				}

				rtcin <- msg_
			})
			break
		}
	})
	dc.OnClose(func() {
		fmt.Printf("Close SSH socket")
	})
}

const (
	DEVWIDTH    = 1920
	DEVHEIGHT   = 1080
	SCREENPARAM = 4096
)

//kcom3 HID 模拟模块接口
//鼠标数据 绝对坐标
//字节: byte1 byet2 byte3 byte01 byte02 byte03  byte04 byte05   byte06
//head: 0x57  0xAB  0x04           低字节在前，高字节在后
//鼠标                    按键    X轴绝对位移值   Y轴绝对位移值     滚轮(0x01-0x07 表示向上滚齿数 0x81-0xFF表)
//0x01 左键按下 0x02 右键按下 0x04 中键按下
//普通键盘数据
//0x57 0xAB 0x01 8 字节标准键盘数据
//8 字节标准键盘数据：
//BYTE1 BYTE2 BYTE3 BYTE4 BYTE5 BYTE6 BYTE7 BYTE8
//定义分别是：
/*
BYTE1 --
|--bit0: Left Control 是否按下，按下为 1
|--bit1: Left Shift 是否按下，按下为 1
|--bit2: Left Alt 是否按下，按下为 1
|--bit3: Left GUI 是否按下，按下为 1
|--bit4: Right Control 是否按下，按下为 1
|--bit5: Right Shift 是否按下，按下为 1
|--bit6: Right Alt 是否按下，按下为 1
|--bit7: Right GUI 是否按下，按下为 1
BYTE2 -- 暂不清楚，有的地方说是保留位，设置为 00 即可
BYTE3--BYTE8 -- 这六个为普通按键
*/
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
		case EVENT_MOUSEMOVE:
			fmt.Println("mousemove", hid.Data)
			var mouse MouseData
			json.Unmarshal([]byte(hid.Data.(string)), &mouse)
			//mouse := hid.Data.(MouseData)
			var hiddata []byte
			hiddata = append(hiddata, 0x57)
			hiddata = append(hiddata, 0xAB)
			hiddata = append(hiddata, 0x04)
			if mouse.IsDown == 1 {
				if mouse.IsLeft == 1 {
					hiddata = append(hiddata, 0x01)
				} else if mouse.IsRight == 1 {
					hiddata = append(hiddata, 0x02)
				} else if mouse.IsMiddle == 1 {
					hiddata = append(hiddata, 0x04)
				}
			} else {
				hiddata = append(hiddata, 0x00)
			}
			xx := mouse.X * DEVWIDTH / mouse.Width
			yy := mouse.Y * DEVHEIGHT / mouse.Height
			xx = xx * SCREENPARAM / DEVWIDTH
			yy = yy * SCREENPARAM / DEVHEIGHT

			hiddata = append(hiddata, uint8(xx))
			hiddata = append(hiddata, uint8(xx>>8))

			hiddata = append(hiddata, uint8(yy))
			hiddata = append(hiddata, uint8(yy>>8))

			hiddata = append(hiddata, uint8(0x00))
			fmt.Println(hiddata)
			//hidd := ]byte{0x57, 0xAB, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
			//hidd := ]byte{0x57, 0xAB, 0x04, 0x00, 0x11, 0x08, 0x68, 0x07, 0x00}
			HIDserialHandler(hiddata)
			//HIDserialHandler(hiddata)
			//json.Marshal(hid.Data.(MouseData),&mouse)
		case EVENT_MOUSEDOWN:
			fmt.Println("mousedown", hid.Data)
			var mouse MouseData
			json.Unmarshal([]byte(hid.Data.(string)), &mouse)
			//mouse := hid.Data.(MouseData)
			var hiddata []byte
			hiddata = append(hiddata, 0x57)
			hiddata = append(hiddata, 0xAB)
			hiddata = append(hiddata, 0x04)
			if mouse.IsDown == 1 {
				if mouse.IsLeft == 1 {
					hiddata = append(hiddata, 0x01)
				} else if mouse.IsRight == 1 {
					hiddata = append(hiddata, 0x02)
				} else if mouse.IsMiddle == 1 {
					hiddata = append(hiddata, 0x04)
				}
			} else {
				hiddata = append(hiddata, 0x00)
			}

			xx := mouse.X * DEVWIDTH / mouse.Width
			yy := mouse.Y * DEVHEIGHT / mouse.Height
			xx = xx * SCREENPARAM / DEVWIDTH
			yy = yy * SCREENPARAM / DEVHEIGHT

			hiddata = append(hiddata, uint8(xx))
			hiddata = append(hiddata, uint8(xx>>8))

			hiddata = append(hiddata, uint8(yy))
			hiddata = append(hiddata, uint8(yy>>8))
			hiddata = append(hiddata, 0x00)

			fmt.Println(hiddata)
			//hidd := ]byte{0x57, 0xAB, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
			//hidd := ]byte{0x57, 0xAB, 0x04, 0x00, 0x11, 0x08, 0x68, 0x07, 0x00}
			HIDserialHandler(hiddata)
			//HIDserialHandler(hiddata)
			//json.Marshal(hid.Data.(MouseData),&mouse)
		case EVENT_MOUSEUP:
			fmt.Println("mousemove", hid.Data)
			var mouse MouseData
			json.Unmarshal([]byte(hid.Data.(string)), &mouse)
			//mouse := hid.Data.(MouseData)
			var hiddata []byte
			hiddata = append(hiddata, 0x57)
			hiddata = append(hiddata, 0xAB)
			hiddata = append(hiddata, 0x04)
			hiddata = append(hiddata, 0x00)
			xx := mouse.X * DEVWIDTH / mouse.Width
			yy := mouse.Y * DEVHEIGHT / mouse.Height
			xx = xx * SCREENPARAM / DEVWIDTH
			yy = yy * SCREENPARAM / DEVHEIGHT

			hiddata = append(hiddata, uint8(xx))
			hiddata = append(hiddata, uint8(xx>>8))

			hiddata = append(hiddata, uint8(yy))
			hiddata = append(hiddata, uint8(yy>>8))
			hiddata = append(hiddata, 0x00)
			fmt.Println(hiddata)
			//hidd := ]byte{0x57, 0xAB, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
			//hidd := ]byte{0x57, 0xAB, 0x04, 0x00, 0x11, 0x08, 0x68, 0x07, 0x00}
			HIDserialHandler(hiddata)
			//HIDserialHandler(hiddata)
			//json.Marshal(hid.Data.(MouseData),&mouse)
		case EVENT_KEYDOWN:
			fmt.Println("key", hid.Data)
			//key := hid.Data.(KeyData)
			var key KeyData
			json.Unmarshal([]byte(hid.Data.(string)), &key)
			var hiddata []byte
			hiddata = append(hiddata, 0x57)
			hiddata = append(hiddata, 0xAB)
			hiddata = append(hiddata, 0x01)
			/*
				keyfunc, err := strconv.Atoi(key.FuncKey)
				if err != nil {
					return
				}
			*/

			/*keycode, err := strconv.Atoi(key.KeyCode)
			if err != nil {
				return
			}
			*/
			var keycode HidKeyCode
			switch key.KeyCode {
			case 13:
				keycode = HidKeyCode{0x28, false}
			case 8:
				keycode = HidKeyCode{0x2A, false}
			case 182:
				keycode = HidKeyCode{0x2E, false}
			default:
				keycode = hidkeymap[key.KeyCode]
			}
			if keycode.Shift {
				funckey := key.FuncKey | (1 << 1)
				hiddata = append(hiddata, byte(funckey))
			} else {
				hiddata = append(hiddata, byte(key.FuncKey))

			}
			hiddata = append(hiddata, 0x00)

			hiddata = append(hiddata, byte(keycode.KeyCode))
			hiddata = append(hiddata, 0x00)
			hiddata = append(hiddata, 0x00)
			hiddata = append(hiddata, 0x00)
			hiddata = append(hiddata, 0x00)
			hiddata = append(hiddata, 0x00)
			fmt.Println(hiddata)
			HIDserialHandler(hiddata)
			hidd := []byte{0x57, 0xAB, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
			//time.Sleep(15)
			HIDserialHandler(hidd)
		}
		//fmt.Println("HID", hid)
	})
	dc.OnClose(func() {
		fmt.Printf("Close Control socket")
	})
}

//HTTPAPIServerStreamWebRTC stream video over WebRTC
func RTSP2StreamWebRTC(msg Message) {
	if !Config.ext(msg.Suuid) {
		log.Println("Stream Not Found")
		//没找到则添加
		//return
	}
	Config.RunIFNotRun(msg.Suuid)
	codecs := Config.coGe(msg.Suuid)
	if codecs == nil {
		log.Println("Stream Codec Not Found")
		return
	}
	var AudioOnly bool
	if len(codecs) == 1 && codecs[0].Type().IsAudio() {
		AudioOnly = true
	}
	muxerWebRTC := webrtcdeep.NewMuxer(webrtcdeep.Options{ICEServers: msg.IceServer, PortMin: Config.GetWebRTCPortMin(), PortMax: Config.GetWebRTCPortMax()})
	answer, err := muxerWebRTC.WriteHeader(codecs, enc.Encode(msg.RtcSession))
	if err != nil {
		log.Println("WriteHeader", err)
		return
	}
	/*
		muxerWebRTC.OnDataChannel(func(dc *webrtc.DataChannel) {
			if dc.Label() == "SSH" {
				sshDataChannelHandler(dc)
			}
			if dc.Label() == "Control" {
				controlDataChannelHandler(dc)
			}
			if dc.Label() == "Serial" {
				serialDataChannelHandler(dc)
			}
			if dc.Label() == "HID" {
				HIDDataChannelHandler(dc)
			}
		})
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
	go func() {
		cid, ch := Config.clAd(msg.Suuid)
		defer Config.clDe(msg.Suuid, cid)
		defer muxerWebRTC.Close()
		var videoStart bool
		noVideo := time.NewTimer(10 * time.Second)
		for {
			select {
			case <-noVideo.C:
				log.Println("noVideo")
				return
			case pck := <-ch:
				if pck.IsKeyFrame || AudioOnly {
					noVideo.Reset(10 * time.Second)
					videoStart = true
				}
				if !videoStart && !AudioOnly {
					continue
				}
				err = muxerWebRTC.WritePacket(pck)
				if err != nil {
					log.Println("WritePacket", err)
					return
				}
			}
		}
	}()
}
