package kvm

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/pion/rtsp-bench/server/config"
	"github.com/tarm/serial"
)

var (
	inputmsgChans  chan PublishMsg //prompb.WriteRequest //multi node one chan
	outputmsgChans chan PublishMsg //prompb.WriteRequest //multi node one chan

	hidmsgChans chan PublishMsg //prompb.WriteRequest //multi node one chan
	//outputmsgChans chan PublishMsg //prompb.WriteRequest //multi node one chan
)

var TOPIC = make(map[string]string)

func HIDSerailTask() {
	inputmsgChans = make(chan PublishMsg, 100)
	outputmsgChans = make(chan PublishMsg, 100)
	var HIDserialport *serial.Port
	var err error

	c := &serial.Config{Name: config.Config.KVM.HIDPort, Baud: config.Config.KVM.HIDBaudRate}
	//打开串口

	HIDserialport, err = serial.OpenPort(c)

	if err != nil {
		log.Fatal(err)

	}

	for {
		select {
		case msg := <-inputmsgChans:
			//设置串口编号
			if msg.Topic == "HIDData" {
				command := msg.Msg.([]byte)

				// 写入屏幕HID数据

				fmt.Println("HIDDev:", time.Now().UnixNano(), command)

				_, err := HIDserialport.Write(command)

				if err != nil {
					//log.Fatal(err)
					fmt.Println("hid error", err.Error())

				}
				time.Sleep(time.Millisecond * 15)
			} else if msg.Topic == "ExitHID" {
				return
			}

		}
	}
}
func SerailTask() {
	inputmsgChans = make(chan PublishMsg, 100)
	outputmsgChans = make(chan PublishMsg, 100)
	var serialport *serial.Port
	var err error
	for {
		select {
		case msg := <-inputmsgChans:
			//设置串口编号
			if msg.Topic == "OpenSerial" {
				c := &serial.Config{Name: config.Config.KVM.HIDPort, Baud: config.Config.KVM.HIDBaudRate}

				//打开串口

				serialport, err = serial.OpenPort(c)

				if err != nil {
					log.Fatal(err)

				}
			} else if msg.Topic == "WriteData" {
				command := msg.Msg.(bytes.Buffer)

				// 写入HID 键盘鼠标数据

				fmt.Println("串口数据", command.Bytes())

				_, err := serialport.Write(command.Bytes())

				if err != nil {
					log.Fatal(err)

				}

				buf := make([]byte, 128)

				n, err := serialport.Read(buf)

				log.Printf("读取窗口信息 %s", buf[:n])

				if err != nil {
					log.Fatal(err)

				}

				log.Printf("%q", buf[:n])
				msg := PublishMsg{
					Topic: "RespData",
					Msg:   buf[:n],
				}
				outputmsgChans <- msg

			}

		}
	}
}
func HIDserialHandler(data []byte) {
	msg := PublishMsg{
		Topic: "HIDData",
		Msg:   data,
	}
	inputmsgChans <- msg
}
func HIDserialOpen() {
	msg := PublishMsg{
		Topic: "OpenSerial",
		Msg:   nil,
	}
	inputmsgChans <- msg
}
func serialHandler(data []byte) string {
	msg := PublishMsg{
		Topic: "WriteData",
		Msg:   data,
	}
	inputmsgChans <- msg
	fmt.Println(msg)
	return "writedata ok"
}
