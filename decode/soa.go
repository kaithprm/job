package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"io"
	"log"
	"os"
	"os/signal"
	"time"
)

// --------session layer--------
// type: 用于标识数据包的类型
const (
	// SESSION_TYPE_REQUIRED_ACK 需要对端返回应答
	SESSION_TYPE_REQUIRED_ACK uint8 = 0x01

	// SESSION_TYPE_NOT_REQUIRED_ACK 不需要对端返回应答
	SESSION_TYPE_NOT_REQUIRED_ACK uint8 = 0x02

	// SESSION_TYPE_ACK 成功的应答
	SESSION_TYPE_ACK uint8 = 0x04

	// SESSION_TYPE_NACK 失败的应答
	SESSION_TYPE_NACK uint8 = 0x08
)

// error code:会话层通信错误码,type = 0x08时有效,默认值为0
const (
	// SESSION_CODE_RESULT_OK 成功的应答
	SESSION_CODE_RESULT_OK uint8 = 0x00

	// SESSION_CODE_RESULT_NOT_OK 尝试发送给下家时检测到失败,原因不明
	SESSION_CODE_RESULT_NOT_OK uint8 = 0x01

	// SESSION_CODE_RESULT_NOT_REACHABLE 尝试发送给下家时检测到无法连接到对方,如发现不到服务,无法连接等
	SESSION_CODE_RESULT_NOT_REACHABLE uint8 = 0x02

	// SESSION_CODE_RESULT_CONNECTION_RETRY_OK 建立连接失败后重连成功
	SESSION_CODE_RESULT_CONNECTION_RETRY_OK uint8 = 0x03

	// other(待扩展)
)

// --------message layer--------
// message type:消息体类型
const (

	// R/R调用
	MESSAGE_TYPE_RR_CALL uint8 = 0x00

	// F/F调用
	MESSAGE_TYPE_FF_CALL uint8 = 0x01

	// 事件通知
	MESSAGE_TYPE_EVENT_NOTIFICATION uint8 = 0x02

	// 事件订阅
	MESSAGE_TYPE_EVENT_SUBSCRIPTION uint8 = 0x04

	// 取消事件订阅
	MESSAGE_TYPE_CANCEL_EVENT_SUBSCRIPTION uint8 = 0x08

	// R/R应答
	MESSAGE_TYPE_RR_RESPONSE uint8 = 0x80

	// 事件订阅应答
	MESSAGE_TYPE_EVENT_SUBSCRIPTION_RESPONSE uint8 = 0x44

	// 取消事件订阅应答
	MESSAGE_TYPE_CANCEL_EVENT_SUBSCRIPTION_RESPONSE uint8 = 0x88

	// 服务同步,预留
	MESSAGE_TYPE_SERVICE_SYNCHRONIZATION uint8 = 0x10

	// 服务同步应答,预留
	MESSAGE_TYPE_SERVICE_SYNCHRONIZATION_RESPONSE uint8 = 0x90
)

// return code:应答返回码,用于标记服务调用的异常,返回码错误代表body不是业务端的应答
const (

	// MESSAGE_CODE_OK No error occurred
	MESSAGE_CODE_OK uint8 = 0x00

	// MESSAGE_CODE_NOT_OK	An unspecified error occurred
	MESSAGE_CODE_NOT_OK uint8 = 0x01

	// MESSAGE_CODE_UNKNOWN_SERVICE The requested Service ID is unknown.
	MESSAGE_CODE_UNKNOWN_SERVICE uint8 = 0x02

	// MESSAGE_CODE_UNKNOWN_METHOD The requested Method ID is unknown. Service ID is known.
	MESSAGE_CODE_UNKNOWN_METHOD uint8 = 0x03

	// MESSAGE_CODE_NOT_READY Service ID and Method ID are known. Application not running.
	MESSAGE_CODE_NOT_READY uint8 = 0x04

	// MESSAGE_CODE_NOT_REACHABLE System running the service is not reachable (internal error code only).
	MESSAGE_CODE_NOT_REACHABLE uint8 = 0x05

	// MESSAGE_CODE_TIMEOUT A timeout occurred (internal error code only).
	MESSAGE_CODE_TIMEOUT uint8 = 0x06

	// MESSAGE_CODE_WRONG_PROTOCOL_VERSION Version of SOME/IP protocol not supported
	MESSAGE_CODE_WRONG_PROTOCOL_VERSION uint8 = 0x07

	// MESSAGE_CODE_WRONG_INTERFACE_VERSION Interface version mismatch
	MESSAGE_CODE_WRONG_INTERFACE_VERSION uint8 = 0x08

	// MESSAGE_CODE_MALFORMED_MESSAGE Deserialization error, so that payload cannot be deserialized.
	MESSAGE_CODE_MALFORMED_MESSAGE uint8 = 0x09

	// MESSAGE_CODE_WRONG_MESSAGE_TYPE	An unexpected message type was received (e.g. REQUEST_NO_RETURN for a method defined as REQUEST.)
	MESSAGE_CODE_WRONG_MESSAGE_TYPE uint8 = 0x0a

	// 0x0b - 0x1f	RESERVED

	// MESSAGE_CODE_SERVICE_DISCOVERY_FAILURE 服务发现失败,payload中添加service id和instance id
	MESSAGE_CODE_SERVICE_DISCOVERY_FAILURE uint8 = 0x20
)

// SessionPacket 会话层
// 需要根据timesTamp划定可变长的切片
type SessionPacket struct {
	Type       uint8
	ErrorCode  uint8
	SequenceId uint8
	TimesTamp  uint8 //4bit
	Ttl        uint8 //4bit
	//--------扩展--------
	Time []uint32
}

type DistributePacket struct {
	ProtocolVersion uint8
	MessageNumber   uint8
	MessageFormat   uint8  // 2bit
	CombineNeeded   uint8  // 1bit
	Dummy           uint16 // 13bit,reserved
}

type MessagePacket struct {
	ServiceID        uint16
	InstanceID       uint16
	TypeId           uint16
	MessageID        uint16 // 12bit
	TimesTamp        uint8  // 3bit
	HasExtension     uint8  // 1bit
	ServiceVersion   uint8  // reserved
	InterfaceVersion uint8  //reserved
	MessageType      uint8
	ReturnCode       uint8
	PayloadLength    uint32
	Payload          []uint8
	//--------扩展--------
	Time             []uint32
	ExtensionVersion uint16
	FieldNumber      uint8
	Field            []uint8 // 扩展字段，长度固定，根据业务自定义 8bit
}

// Packet 总的packet 将几个层级合并
type Packet struct {
	SessionPacket    SessionPacket
	DistributePacket DistributePacket
	MessagePacket    []MessagePacket
}

func Decode(reader io.Reader) (*Packet, error) {
	bufReader := bufio.NewReader(reader)

	// --------session layer--------
	// read session type 8bit
	sessionType, err := bufReader.ReadByte()
	if err != nil {
		return nil, err
	}

	// read session error code 8bit
	sessionErrorCode, err := bufReader.ReadByte()
	if err != nil {
		return nil, err
	}

	// read sequenceId 8bit
	sequenceId, err := bufReader.ReadByte()
	if err != nil {
		return nil, err
	}

	// read timestamp,ttl 4bit
	ttl, timestamp, err := read4(bufReader)
	if err != nil {
		return nil, err
	}

	//judge sessionTimesTamp 根据timestamp数量 给time分配空间
	time := make([]uint32, 0, timestamp)

	for i := 0; i < int(timestamp); i++ {
		value, err := read32(bufReader)
		if err != nil {
			// 处理错误
		}
		time = append(time, value)
	}

	// --------distribute layer--------
	// read protocolVersion 8bit
	protocolVersion, err := bufReader.ReadByte()
	if err != nil {
		return nil, err
	}

	// read messageNumber   uint8
	messageNumber, err := bufReader.ReadByte()
	if err != nil {
		return nil, err
	}

	// read messageFormat,combineNeeded,dummy
	messageFormat, combineNeeded, dummy, err := read13(bufReader)
	if err != nil {
		return nil, err
	}

	// --------message layer--------
	// 创建包含messageNumber个MessagePacket的切片
	messagePackets := make([]MessagePacket, messageNumber)

	// 从1开始遍历
	for j := 0; j < int(messageNumber); j++ {

		// read serviceID 16bit
		serviceID, err := read16(bufReader)
		if err != nil {
			return nil, err
		}

		// read InstanceID 16bit
		instanceID, err := read16(bufReader)
		if err != nil {
			return nil, err
		}

		// read eventGroupID 16bit
		eventGroupID, err := read16(bufReader)
		if err != nil {
			return nil, err
		}

		// read messageID, messageTimesTamp, hasExtension
		messageID, messageTimesTamp, hasExtension, err := read12(bufReader)
		if err != nil {
			return nil, err
		}

		// read serviceVersion
		serviceVersion, err := bufReader.ReadByte()
		if err != nil {
			return nil, err
		}

		//read interfaceVersion
		interfaceVersion, err := bufReader.ReadByte()
		if err != nil {
			return nil, err
		}

		//read messageType
		messageType, err := bufReader.ReadByte()
		if err != nil {
			return nil, err
		}

		//read returnCode
		returnCode, err := bufReader.ReadByte()
		if err != nil {
			return nil, err
		}

		//read payloadLength
		payloadLength, err := read32(bufReader)
		if err != nil {
			return nil, err
		}

		//judge messageTimesTamp 根据timestamp数量 给time分配空间
		messageTime := make([]uint32, 0, messageTimesTamp)

		for i := 0; i < int(messageTimesTamp); i++ {
			value, err := read32(bufReader)
			if err != nil {
				return nil, err
			}
			time = append(time, value)
		}

		// judge payloadLength 根据payloadLength读payload大小
		var payload []uint8
		for k := 0; k < int(payloadLength); k++ {
			b, err := bufReader.ReadByte()
			if err != nil {
				return nil, err
			}
			payload = append(payload, b)
		}

		//judge HasExtension 有无扩展
		var extensionVersion uint16
		var fieldNumber uint8
		var field []uint8
		if hasExtension != 0 {
			extensionVersion, err = read16(bufReader)
			if err != nil {
				return nil, err
			}

			//read returnCode
			fieldNumber, err := bufReader.ReadByte()
			if err != nil {
				return nil, err
			}

			//judge field 根据fieldNumber量 给time分配空间
			field := make([]uint8, 0, fieldNumber) // 使用make函数创建一个长度为0，容量为fieldNumber的uint8切片

			for i := 0; i < int(fieldNumber); i++ {
				value, err := bufReader.ReadByte()
				if err != nil {
					// 处理错误
				}
				field = append(field, value) // 将读取的字节追加到field切片中
			}
		}
		messagePackets[j] = MessagePacket{
			ServiceID:        serviceID,
			InstanceID:       instanceID,
			TypeId:           eventGroupID,
			MessageID:        messageID,
			TimesTamp:        messageTimesTamp,
			HasExtension:     hasExtension,
			ServiceVersion:   serviceVersion,
			InterfaceVersion: interfaceVersion,
			MessageType:      messageType,
			ReturnCode:       returnCode,
			PayloadLength:    payloadLength,
			Payload:          payload,
			//--------扩展--------
			Time:             messageTime,
			ExtensionVersion: extensionVersion,
			FieldNumber:      fieldNumber,
			Field:            field,
		}
	}
	test := &Packet{
		SessionPacket: SessionPacket{
			Type:       sessionType,
			ErrorCode:  sessionErrorCode,
			SequenceId: sequenceId,
			TimesTamp:  timestamp,
			Ttl:        ttl,
			Time:       time,
		},
		DistributePacket: DistributePacket{
			ProtocolVersion: protocolVersion,
			MessageNumber:   messageNumber,
			MessageFormat:   messageFormat,
			CombineNeeded:   combineNeeded,
			Dummy:           dummy,
		},
		MessagePacket: messagePackets,
	}
	return test, nil
}

// 读取4bit方法
func read4(reader *bufio.Reader) (uint8, uint8, error) {
	buf := make([]byte, 1)
	if _, err := reader.Read(buf); err != nil {
		return 0, 0, err
	}
	upper := buf[0] & 0x0F //获取前四位
	lower := buf[0] >> 4   // 获取后四位

	return upper, lower, nil
}

// 读取16bit方法
func read16(reader *bufio.Reader) (uint16, error) {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf), nil
}

// 读取32bit方法
func read32(reader *bufio.Reader) (uint32, error) {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf), nil
}

// 读取2，1，13方法
func read13(reader *bufio.Reader) (uint8, uint8, uint16, error) {
	buf := make([]byte, 2)
	if _, err := reader.Read(buf); err != nil {
		return 0, 0, 0, err
	}
	bit2 := (buf[0] & 0xC0) >> 6
	bit1 := (buf[0] & 0x20) >> 5
	// 获取第一个字节的剩下的5个bit
	remaining := uint16(buf[0] & 0x1F)
	// 合并剩下的5个bit和第二个字节的8个bit
	bit13 := (remaining << 8) | uint16(buf[1])
	return bit2, bit1, bit13, nil
}

// 读取12，3，1方法
func read12(reader *bufio.Reader) (uint16, uint8, uint8, error) {
	buf := make([]byte, 2)
	if _, err := reader.Read(buf); err != nil {
		return 0, 0, 0, err
	}
	// 获取第二个字节的前四个bit
	remaining := uint16(buf[1] & 0xF0)
	// 合并第一个byte和第二个字节的前4个bit
	bit12 := uint16(buf[0]) | (remaining << 4)
	bit3 := (buf[1] & 0xE) >> 1
	bit1 := buf[1] & 0x1
	return bit12, bit3, bit1, nil
}

func Encode(p *Packet) ([]byte, error) {

	buffer := bytes.NewBuffer(make([]byte, 0, 128)) // 这个容量大小如何设置

	// --------session layer--------
	err := buffer.WriteByte(p.SessionPacket.Type)
	if err != nil {
		return nil, err
	}

	err = buffer.WriteByte(p.SessionPacket.ErrorCode)
	if err != nil {
		return nil, err
	}

	err = buffer.WriteByte(p.SessionPacket.SequenceId)
	if err != nil {
		return nil, err
	}

	err = buffer.WriteByte(merge4Bits(p.SessionPacket.TimesTamp, p.SessionPacket.Ttl))
	if err != nil {
		return nil, err
	}

	// extension,写入time扩展 大端
	for i := 0; i < int(p.SessionPacket.TimesTamp) && i < len(p.SessionPacket.Time); i++ {
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, p.SessionPacket.Time[i])
		buffer.Write(buf)
	}

	// --------distribute layer--------
	err = buffer.WriteByte(p.DistributePacket.ProtocolVersion)
	if err != nil {
		return nil, err
	}

	err = buffer.WriteByte(p.DistributePacket.MessageNumber)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buffer, binary.BigEndian, merge13Bits(p.DistributePacket.MessageFormat, p.DistributePacket.CombineNeeded, p.DistributePacket.Dummy))
	if err != nil {
		return nil, err
	}

	// --------message layer--------
	// 根据messageNumber 写入messageHeader
	for j := 0; j < int(p.DistributePacket.MessageNumber); j++ {

		// write service id
		err = binary.Write(buffer, binary.BigEndian, p.MessagePacket[j].ServiceID)
		if err != nil {
			return nil, err
		}

		// write instance id
		err = binary.Write(buffer, binary.BigEndian, p.MessagePacket[j].InstanceID)
		if err != nil {
			return nil, err
		}

		// write eventGroup id
		err = binary.Write(buffer, binary.BigEndian, p.MessagePacket[j].TypeId)
		if err != nil {
			return nil, err
		}

		// write merge12 bits
		err = binary.Write(buffer, binary.BigEndian, merge12Bits(p.MessagePacket[j].MessageID, p.MessagePacket[j].TimesTamp, p.MessagePacket[j].HasExtension))
		if err != nil {
			return nil, err
		}

		// write service version
		err = buffer.WriteByte(p.MessagePacket[j].ServiceVersion)
		if err != nil {
			return nil, err
		}

		// write interface version
		err = buffer.WriteByte(p.MessagePacket[j].InterfaceVersion)
		if err != nil {
			return nil, err
		}

		// write message type
		err = buffer.WriteByte(p.MessagePacket[j].MessageType)
		if err != nil {
			return nil, err
		}

		// write return code
		err = buffer.WriteByte(p.MessagePacket[j].ReturnCode)
		if err != nil {
			return nil, err
		}

		// write payload length
		err = binary.Write(buffer, binary.BigEndian, p.MessagePacket[j].PayloadLength)
		if err != nil {
			return nil, err
		}

		// 根据payload length写入payload
		for k := 0; k < int(p.MessagePacket[j].PayloadLength); k++ {
			err = buffer.WriteByte(p.MessagePacket[j].Payload[k])
			if err != nil {
				return nil, err
			}

		}

		// extension 扩展time
		for i := 0; i < int(p.MessagePacket[j].TimesTamp) && i < len(p.MessagePacket[j].Time); i++ {
			buf := make([]byte, 4)
			binary.BigEndian.PutUint32(buf, p.MessagePacket[j].Time[i])
			_, err = buffer.Write(buf)
			if err != nil {
				return nil, err
			}
		}

		// write extension version
		err = binary.Write(buffer, binary.BigEndian, p.MessagePacket[j].ExtensionVersion)
		if err != nil {
			return nil, err
		}

		// write field number
		err = buffer.WriteByte(p.MessagePacket[j].FieldNumber)
		if err != nil {
			return nil, err
		}

		// extension 根据fieldNumber扩展field
		for i := 0; i < int(p.MessagePacket[j].FieldNumber); i++ {
			err = buffer.WriteByte(p.MessagePacket[j].Field[i])
			if err != nil {
				return nil, err
			}
		}
	}
	return buffer.Bytes(), nil
}

// 合并两个4bit
func merge4Bits(val1, val2 uint8) uint8 {
	// 将val1向左移动4位，然后与val2进行按位或操作
	result := (val1 << 4) | val2
	return result
}

// 合并2，1，13bit
func merge13Bits(val2 uint8, val1 uint8, val13 uint16) uint16 {
	// 扩容会将uint8的值直接拷贝到uint16中，并将高位补0
	bit := (val2 << 1) | val1
	result := uint16(bit)<<13 | val13
	return result
}

// 合并12，3，1bit
func merge12Bits(val12 uint16, val3 uint8, val1 uint8) uint16 {
	// 扩容会将uint8的值直接拷贝到uint16中，并将高位补0
	bit := (val3 << 1) | val1
	result := val12<<4 | uint16(bit)
	return result
}

// mqtt client
func onMessageReceived(client mqtt.Client, msg mqtt.Message) {
	//fmt.Printf("订阅: 当前话题是 [%s]; 信息是 [%s] \n", msg.Topic(), msg.Payload())
	var buf bytes.Buffer
	buf.Write(msg.Payload())
	reader := &buf
	p1, err := Decode(reader)
	if err != nil {
		fmt.Println("decode error is ", err)
	}
	fmt.Println("type is :", p1.SessionPacket.Type)
	fmt.Println("SequenceId :", p1.SessionPacket.SequenceId)
	//fmt.Println("ServiceID :", p1.messagePacket[].ServiceID)
}

func main() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker("47.94.135.4:11883")
	opts.SetClientID("go-mqtt-client")
	opts.SetDefaultPublishHandler(onMessageReceived) // 设置默认的消息处理函数

	// 创建 MQTT 客户端实例
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	// 在连接成功后进行订阅发布，做包操作
	go func() {
		// 订阅主题
		if token := client.Subscribe("/v2c/sync/SGL0000413032020F", 0, nil); token.Wait() && token.Error() != nil {
			log.Fatal(token.Error())
		}
		//做包
		var a = []uint32{1, 2, 3}
		var b = []uint32{1, 2, 3, 4, 5}
		var c = []uint8{1, 2, 3}

		p := Packet{
			SessionPacket: SessionPacket{
				Type:       1,
				ErrorCode:  10,
				SequenceId: 1,
				TimesTamp:  0,
				Ttl:        1,
				Time:       a,
			},
			DistributePacket: DistributePacket{
				ProtocolVersion: 11,
				MessageNumber:   1,
				MessageFormat:   1,
				CombineNeeded:   1,
				Dummy:           123,
			},
			MessagePacket: []MessagePacket{{
				ServiceID:        0x2000,
				InstanceID:       1,
				TypeId:           0x1001,
				MessageID:        123,
				TimesTamp:        0,
				HasExtension:     0,
				ServiceVersion:   12,
				InterfaceVersion: 1,
				MessageType:      0,
				ReturnCode:       0,
				PayloadLength:    0,
				//--------扩展--------
				Time:             b,
				ExtensionVersion: 1234,
				FieldNumber:      0,
				Field:            c,
			}},
		}
		result, err := Encode(&p)

		if err != nil {
			println("encode error :", err)
		}
		// 发布消息

		// 将字节片转换为16进制格式的字符串
		token := client.Publish("/c2v/sync/SGL0000413032020F", 0, false, result)
		token.Wait()

		time.Sleep(time.Second)

	}()

	// 等待退出信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	// 断开与 MQTT 服务器的连接
	client.Disconnect(250)
}
