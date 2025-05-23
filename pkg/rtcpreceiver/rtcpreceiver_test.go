package rtcpreceiver

import (
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func uint32Ptr(v uint32) *uint32 {
	return &v
}

func TestRTCPReceiverBase(t *testing.T) {
	done := make(chan struct{})

	rr := &RTCPReceiver{
		ClockRate: 90000,
		LocalSSRC: uint32Ptr(0x65f83afb),
		Period:    500 * time.Millisecond,
		TimeNow: func() time.Time {
			return time.Date(2008, 0o5, 20, 22, 15, 22, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 0x65f83afb,
				Reports: []rtcp.ReceptionReport{
					{
						SSRC:               0xba9da416,
						LastSequenceNumber: 947,
						LastSenderReport:   0x887a17ce,
						Delay:              2 * 65536,
					},
				},
			}, pkt)
			close(done)
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     0xafb45733,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessSenderReport(&srPkt, ts)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 946,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 947,
			Timestamp:      0xafb45733 + 90000,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
	err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	<-done
}

func TestRTCPReceiverOverflow(t *testing.T) {
	done := make(chan struct{})

	rr := &RTCPReceiver{
		ClockRate: 90000,
		LocalSSRC: uint32Ptr(0x65f83afb),
		Period:    250 * time.Millisecond,
		TimeNow: func() time.Time {
			return time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 0x65f83afb,
				Reports: []rtcp.ReceptionReport{
					{
						SSRC:               0xba9da416,
						LastSequenceNumber: 1 << 16,
						LastSenderReport:   0x887a17ce,
						Delay:              1 * 65536,
					},
				},
			}, pkt)
			close(done)
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	time.Sleep(400 * time.Millisecond)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessSenderReport(&srPkt, ts)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0xffff,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0000,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	<-done
}

func TestRTCPReceiverPacketsLost(t *testing.T) {
	done := make(chan struct{})

	rr := &RTCPReceiver{
		ClockRate: 90000,
		LocalSSRC: uint32Ptr(0x65f83afb),
		Period:    500 * time.Millisecond,
		TimeNow: func() time.Time {
			return time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 0x65f83afb,
				Reports: []rtcp.ReceptionReport{
					{
						SSRC:               0xba9da416,
						LastSequenceNumber: 0x0122,
						LastSenderReport:   0x887a17ce,
						FractionLost: func() uint8 {
							v := float64(1) / 3
							return uint8(v * 256)
						}(),
						TotalLost: 1,
						Delay:     1 * 65536,
					},
				},
			}, pkt)
			close(done)
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessSenderReport(&srPkt, ts)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0120,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0122,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	<-done
}

func TestRTCPReceiverOverflowPacketsLost(t *testing.T) {
	done := make(chan struct{})

	rr := &RTCPReceiver{
		ClockRate: 90000,
		LocalSSRC: uint32Ptr(0x65f83afb),
		Period:    500 * time.Millisecond,
		TimeNow: func() time.Time {
			return time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 0x65f83afb,
				Reports: []rtcp.ReceptionReport{
					{
						SSRC:               0xba9da416,
						LastSequenceNumber: 1<<16 | 0x0002,
						LastSenderReport:   0x887a17ce,
						FractionLost: func() uint8 {
							v := float64(2) / 4
							return uint8(v * 256)
						}(),
						TotalLost: 2,
						Delay:     1 * 65536,
					},
				},
			}, pkt)
			close(done)
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessSenderReport(&srPkt, ts)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0xffff,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0002,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	<-done
}

func TestRTCPReceiverJitter(t *testing.T) {
	done := make(chan struct{})

	rr := &RTCPReceiver{
		ClockRate: 90000,
		LocalSSRC: uint32Ptr(0x65f83afb),
		Period:    500 * time.Millisecond,
		TimeNow: func() time.Time {
			return time.Date(2008, 0o5, 20, 22, 15, 22, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 0x65f83afb,
				Reports: []rtcp.ReceptionReport{
					{
						SSRC:               0xba9da416,
						LastSequenceNumber: 948,
						LastSenderReport:   0x887a17ce,
						Delay:              2 * 65536,
						Jitter:             45000 / 16,
					},
				},
			}, pkt)
			close(done)
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     0xafb45733,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessSenderReport(&srPkt, ts)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 946,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 947,
			Timestamp:      0xafb45733 + 45000,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
	err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 948,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 22, 0, time.UTC)
	err = rr.ProcessPacket(&rtpPkt, ts, false)
	require.NoError(t, err)

	<-done
}
