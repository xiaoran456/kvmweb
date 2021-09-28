#!/bin/bash
mount -t configfs none /sys/kernel/config
cd /sys/kernel/config/usb_gadget
mkdir gadget
cd gadget
ls
 
#含有如下文件和目录
# UDC              bMaxPacketSize0  functions        strings
# bDeviceClass     bcdDevice        idProduct
# bDeviceProtocol  bcdUSB           idVendor
# bDeviceSubClass  configs          os_desc
 
#设置USB协议版本USB2.0
echo 0x0200  > bcdUSB
 
#定义产品的VendorID和ProductID
echo "0x0525"  > idVendor
echo "0xa4ac" > idProduct
 
#实例化"英语"ID：
mkdir strings/0x409
ls strings/0x409
 
#manufacturer    product    serialnumber
 
#将开发商、产品和序列号字符串写入内核：
echo "76543210" > strings/0x409/serialnumber
echo "mkelehk"  > strings/0x409/manufacturer
echo "keyboard_mouse"  > strings/0x409/product
 
#创建一个USB配置实例：
mkdir configs/config.1
ls configs/config.1
 
#MaxPower bmAttributes strings
 
echo 120 > configs/config.1/MaxPower
 
#定义配置描述符使用的字符串
mkdir configs/config.1/strings/0x409
ls configs/config.1/strings/0x409/
 
#configuration
 
echo "hid" >   configs/config.1/strings/0x409/configuration
 
#创建功能实例，需要注意的是，一个功能如果有多个实例的话，扩展名必须用数字编号：
mkdir functions/hid.0
mkdir functions/hid.1
 
#配置hid描述符（根据hid协议或者g_hid.ko对于的源码hid.c的说明）
echo 1 > functions/hid.0/subclass
echo 1 > functions/hid.0/protocol
echo 8 > functions/hid.0/report_length
echo -ne \\x5\\x1\\x9\\x2\\xa1.... > functions/hid.0/report_desc
 
echo 1 > functions/hid.1/subclass
echo 2 > functions/hid.1/protocol
echo 4 > functions/hid.1/report_length
echo -ne \\x5\\x1\\x9\\x2\\xa1.... > functions/hid.1/report_desc
 
#捆绑功能实例到配置config.1
ln -s functions/hid.0 configs/config.1
ln -s functions/hid.1 configs/config.1
 
#查找本机可获得的UDC实例
ls /sys/class/udc/
 
#musb-hdrc.1.auto
 
#将gadget驱动注册到UDC上，插上USB线到电脑上，电脑就会枚举USB设备。
echo "musb-hdrc.1.auto" > UDC
