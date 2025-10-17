# OTP SMS Service

A **simple, generic version of the method I use** for sending OTP/SMS messages with multiple modems. Built with Go and ModemManager (mmcli) for concurrent SMS processing.

**Repository**: [https://github.com/YslamB/otppc.git](https://github.com/YslamB/otppc.git)

## Features

- ✅ Auto-discovers and enables connected modems
- ✅ Concurrent processing with worker pool (3 workers)
- ✅ Message queue with 100-message buffer
- ✅ Graceful shutdown (SIGINT/SIGTERM)
- ✅ Production-ready deployment scripts

## What You Need

- **One PC** (MiniPC, old laptop, or any Linux machine you have)
- **One or more USB modems** (the more modems, the more concurrent SMS you can send)
- ModemManager installed
- Go 1.24.3+

## Quick Start

```bash
git clone https://github.com/YslamB/otppc.git
cd otppc
go build -o otp .
./otp

# Or deploy directly
make deploy
```

## How It Works

The service uses ModemManager CLI (`mmcli`) commands to interact with modems:

1. **List modems**: `mmcli -L`
2. **Enable modem**: `mmcli -m <modemID> --enable`
3. **Create SMS**: `mmcli -m <modemID> --messaging-create-sms="text='...',number='...'"`
4. **Send SMS**: `mmcli -s <smsID> --send`

### Architecture

```
┌──────────────┐
│ enableModems │ ──► Auto-discovers & enables modems (every 15s)
└──────────────┘
       │
       ▼
┌──────────────┐
│  Main Loop   │ ──► Enqueues messages
└──────────────┘
       │
       ▼
┌──────────────┐
│ Message Queue│ ──► Buffer: 100 messages
└──────────────┘
       │
       ▼
┌──────────────┐
│ Worker Pool  │ ──► 3 concurrent workers
└──────────────┘
       │
       ▼
┌──────────────┐
│   sendSMS()  │ ──► Creates & sends via mmcli
└──────────────┘
```

## Configuration

Adjust parameters in the code:

```go
// main.go
modemQueue := make(chan message, 100)  // Queue buffer size
workerCount := 3                       // Number of workers
time.Sleep(15 * time.Second)           // Modem check interval
time.Sleep(10 * time.Second)           // Delay after sending
```

## Deployment

```bash
make deploy
```

Deploys to: `ubuntu@your_ip_address:/var/www/otp`

## Customization

Replace `getMessage()` with your database/API integration:

```go
func getMessage() (message, error) {
    // Fetch from your database/API
    return message{
        PhoneNumber: "+99361041499",
        MessageText: "Your OTP is: 123456",
    }, nil
}
```

## Troubleshooting

```bash
# Check modems
mmcli -L

# Check ModemManager
systemctl status ModemManager

# Check modem status
mmcli -m 0

# List SMS messages
mmcli -m 0 --messaging-list-sms
```

## License

Provided as-is for OTP/SMS service implementations.
