package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// 会话层
// 需要根据timesstamp划定可变长的切片
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
	EventGroupID     uint16
	MessageID        uint16 // 12bit
	TimesTamp        uint8  // 3bit
	HasExtension     uint8  // 1bit
	ServiceVersion   uint8  // reserved
	InterfaceVersion uint8  //reserved
	MessageType      uint8
	ReturnCode       uint8
	PayloadLength    uint32
	//--------扩展--------
	Time             []uint32
	ExtensionVersion uint16
	FieldNumber      uint8
	Field            []uint8 // 扩展字段，长度固定，根据业务自定义 8bit
}

// 总的packet 将几个层级合并
type Packet struct {
	sessionPacket    SessionPacket
	distributePacket DistributePacket
	messagePacket    MessagePacket
}

func Decode(reader io.Reader) (*Packet, error) {
	bufReader := bufio.NewReader(reader)

	// --------session layer--------
	// read session type 8bit
	sessiontype, err := bufReader.ReadByte()
	if err != nil {
		return nil, err
	}

	// read session error code 8bit
	seesionerrorcode, err := bufReader.ReadByte()
	if err != nil {
		return nil, err
	}

	// read sequenceid 8bit
	sequenceid, err := bufReader.ReadByte()
	if err != nil {
		return nil, err
	}

	// read timestamp,ttl 4bit
	ttl, timestamp, err := read4(bufReader)
	if err != nil {
		return nil, err
	}

	//judge sessiontimestamp 根据timestamp数量 给time分配空间
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
			// 处理错误
		}
		time = append(time, value)
	}

	//judge HasExtension 有无扩展
	var extensionVersion uint16
	if hasExtension != 0 {
		extensionVersion, err = read16(bufReader)
		if err != nil {
			return nil, err
		}
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

	test := &Packet{
		sessionPacket: SessionPacket{
			Type:       sessiontype,
			ErrorCode:  seesionerrorcode,
			SequenceId: sequenceid,
			TimesTamp:  timestamp,
			Ttl:        ttl,
			Time:       time,
		},
		distributePacket: DistributePacket{
			ProtocolVersion: protocolVersion,
			MessageNumber:   messageNumber,
			MessageFormat:   messageFormat,
			CombineNeeded:   combineNeeded,
			Dummy:           dummy,
		},
		messagePacket: MessagePacket{
			ServiceID:        serviceID,
			InstanceID:       instanceID,
			EventGroupID:     eventGroupID,
			MessageID:        messageID,
			TimesTamp:        messageTimesTamp,
			HasExtension:     hasExtension,
			ServiceVersion:   serviceVersion,
			InterfaceVersion: interfaceVersion,
			MessageType:      messageType,
			ReturnCode:       returnCode,
			PayloadLength:    payloadLength,
			//--------扩展--------
			Time:             messageTime,
			ExtensionVersion: extensionVersion,
			FieldNumber:      fieldNumber,
			Field:            field,
		},
	}
	fmt.Println(test.sessionPacket.Type)
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

// Encode
func Encode(p *Packet) ([]byte, error) {

	buffer := bytes.NewBuffer(make([]byte, 0, 1024)) // 这个容量大小如何设置

	// --------session layer--------
	err1 := buffer.WriteByte(p.sessionPacket.Type)
	if err1 != nil {
		fmt.Println("write type error :", err1)
		return nil, err1
	}

	err2 := buffer.WriteByte(p.sessionPacket.ErrorCode)
	if err2 != nil {
		fmt.Println("write errorCode error :", err2)
		return nil, err2
	}

	err3 := buffer.WriteByte(p.sessionPacket.SequenceId)
	if err3 != nil {
		fmt.Println("write sequenceId error :", err3)
		return nil, err3
	}

	err4 := buffer.WriteByte(merge4Bits(p.sessionPacket.TimesTamp, p.sessionPacket.Ttl))
	if err4 != nil {
		fmt.Println("merge4Bits error :", err4)
		return nil, err4
	}

	// extension,写入time扩展 大端
	for i := 0; i < int(p.sessionPacket.TimesTamp) && i < len(p.sessionPacket.Time); i++ {
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, p.sessionPacket.Time[i])
		buffer.Write(buf)
	}

	// --------distribute layer--------
	err5 := buffer.WriteByte(p.distributePacket.ProtocolVersion)
	if err5 != nil {
		fmt.Println("write protocol version error :", err5)
		return nil, err5
	}

	err6 := buffer.WriteByte(p.distributePacket.MessageNumber)
	if err6 != nil {
		fmt.Println("write message number error :", err6)
		return nil, err6
	}

	err7 := binary.Write(buffer, binary.BigEndian, merge13Bits(p.distributePacket.MessageFormat, p.distributePacket.CombineNeeded, p.distributePacket.Dummy))
	if err7 != nil {
		fmt.Println("merge13bits error :", err7)
		return nil, err7
	}

	// --------message layer--------
	binary.Write(buffer, binary.BigEndian, p.messagePacket.ServiceID)
	binary.Write(buffer, binary.BigEndian, p.messagePacket.InstanceID)
	binary.Write(buffer, binary.BigEndian, p.messagePacket.EventGroupID)
	binary.Write(buffer, binary.BigEndian, merge12Bits(p.messagePacket.MessageID, p.messagePacket.TimesTamp, p.messagePacket.HasExtension))
	buffer.WriteByte(p.messagePacket.ServiceVersion)
	buffer.WriteByte(p.messagePacket.InterfaceVersion)
	buffer.WriteByte(p.messagePacket.MessageType)
	buffer.WriteByte(p.messagePacket.ReturnCode)
	binary.Write(buffer, binary.BigEndian, p.messagePacket.PayloadLength)
	// extension 扩展time
	for i := 0; i < int(p.messagePacket.TimesTamp) && i < len(p.messagePacket.Time); i++ {
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, p.messagePacket.Time[i])
		buffer.Write(buf)
	}
	binary.Write(buffer, binary.BigEndian, p.messagePacket.ExtensionVersion)
	buffer.WriteByte(p.messagePacket.FieldNumber)

	// extension 根据fieldNumber扩展field
	for i := 0; i < int(p.messagePacket.FieldNumber); i++ {
		err := buffer.WriteByte(p.messagePacket.Field[i])
		if err != nil {
			println("extension field error :", err)
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
