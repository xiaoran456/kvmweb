// external ip :http://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
// mac: http://godoc.org/github.com/j-keck/arping
//      http://golangtc.com/t/52d26aa7320b5237d1000044
// vendor: http://zhidao.baidu.com/question/37072459.html
package kvm

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/franela/goreq"
	"github.com/j-keck/arping"
)

func DiscoveryDevice() *Fing {
	fing := new(Fing)
	fing.Detect()
	fing.Show()
	return fing
}

type Fing struct {
	Devices []*Device
}

type Device struct {
	Ip     string
	Mac    string
	Vendor string
	Type   int
}

func NewDevice(ip, mac, vendor string, t int) *Device {
	device := new(Device)
	device.Ip = ip
	device.Mac = mac
	device.Vendor = vendor
	device.Type = t
	return device
}

func (this *Fing) Detect() {
	// Get own IP
	ip, ownmac, err := ExternalIP()
	if err != nil {
		log.Println(err)
		return
	}
	//vendor := ownmac

	vendor, err := Vendor(ownmac)
	if err != nil {
		log.Println(err)
		return
	}

	this.Devices = append(this.Devices, NewDevice(ip, ownmac, vendor, TYPE_OWN_DEVICE))

	ipFormat := ip[:strings.LastIndex(ip, ".")+1] + "%d"
	for i := 1; i <= 27; i++ {
		nextIp := fmt.Sprintf(ipFormat, i)
		if nextIp != ip {
			hwAddr, duration, err := Mac(nextIp)
			if err == arping.ErrTimeout {
				log.Printf("IP %s is offline.\n", nextIp)
			} else if err != nil {
				log.Printf("IP %s :%s\n", nextIp, err.Error())
			} else {
				log.Printf("%s (%s) %d usec\n", nextIp, hwAddr, duration/1000)

				vendor, err := Vendor(hwAddr.String())
				if err != nil {
					log.Println(err)
					return
				}
				//vendor := hwAddr.String()
				this.Devices = append(this.Devices, NewDevice(nextIp, hwAddr.String(), vendor, TYPE_OTHER_DEVICE))
			}
		}
	}

}

func (this *Fing) Show() {
	fmt.Printf("%3s|%15s|%17s|%20s|%4s\n", "#", "IP", "MAC", "VENDOR", "TYPE")
	for i, device := range this.Devices {
		fmt.Printf("%3d|%15s|%17s|%20s|%4s\n", i, device.Ip, device.Mac, device.Vendor, this.showType(device.Type))
	}
}

func (this *Fing) showType(t int) string {
	switch t {
	case TYPE_OWN_DEVICE:
		return "OWN"
	}
	return ""
}

const (
	TYPE_OWN_DEVICE = iota
	TYPE_OTHER_DEVICE
)

func Vendor(mac string) (string, error) {
	macs := strings.Split(mac, ":")
	if len(macs) != 6 {
		return "", fmt.Errorf("MAC Error: %s", mac)
	}
	mac = strings.Join(macs[0:3], "-")
	return mac, nil
	form := url.Values{}
	form.Add("x", mac)
	form.Add("submit2", "Search!")
	res, err := goreq.Request{
		Method:      "POST",
		Uri:         "http://standards.ieee.org/cgi-bin/ouisearch",
		ContentType: "application/x-www-form-urlencoded",
		UserAgent:   "Cyeam",
		ShowDebug:   true,
		Body:        form.Encode(),
	}.Do()
	if err != nil {
		return "", err
	}
	body, err := res.Body.ToString()
	if err != nil {
		return "", err
	}
	vendor := body[strings.Index(body, strings.ToUpper(mac))+len(mac):]
	vendor = strings.TrimLeft(vendor, "</b>   (hex)")
	vendor = strings.TrimSpace(vendor)
	return strings.Split(vendor, "\n")[0], nil
}

func Mac(ip string) (net.HardwareAddr, time.Duration, error) {
	dstIP := net.ParseIP(ip)
	return arping.Ping(dstIP)
}

func ExternalIP() (string, string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), iface.HardwareAddr.String(), nil
		}
	}
	return "", "", errors.New("are you connected to the network?")
}

/*
func findIP(onNewIP)[]string { //  onNewIp - your listener function for new IPs
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}
	var localIPs map[string]bool
	go func(){
       // regexp.MatchString
	}()
     noop = function () {},
     localIPs = {}, //记录有没有被调用到onNewIP这个listener上
     ipRegex = /([0-9]{1,3}(\.[0-9]{1,3}){3}|[a-f0-9]{1,4}(:[a-f0-9]{1,4}){7})/g,
     key;

   func ipIterate(ip) {
     if (!localIPs[ip]) onNewIP(ip);
     localIPs[ip] = true ;
   }
   pc.createDataChannel( "" ); //create a bogus data channel
   pc.createOffer().then( function (sdp) {
     sdp.sdp.split( '\n' ).forEach( function (line) {
       if (line.indexOf( 'candidate' ) < 0) return ;
       line.match(ipRegex).forEach(ipIterate);
     });
     pc.setLocalDescription(sdp, noop, noop);
   }); // create offer and set local description
   pc.onicecandidate = function (ice) { //listen for candidate events
     if (!ice || !ice.candidate || !ice.candidate.candidate || !ice.candidate.candidate.match(ipRegex)) return ;
     ice.candidate.candidate.match(ipRegex).forEach(ipIterate);
   };
}
*/
/*

var ul = document.createElement( 'ul' );
ul.textContent = 'Your IPs are: '
document.body.appendChild(ul);

function addIP(ip) {
   console.log( 'got ip: ' , ip);
   var li = document.createElement( 'li' );
   li.textContent = ip;
   ul.appendChild(li);
}

findIP(addIP);
*/
