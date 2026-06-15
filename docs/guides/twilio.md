# Twilio Setup

This guide covers configuring Twilio for voice calls and SMS/MMS/RCS messaging with OmniAgent.

## Prerequisites

1. A [Twilio account](https://www.twilio.com/try-twilio) (free trial available)
2. A Twilio phone number with Voice and/or SMS capability
3. A public URL for webhooks (or ngrok for local development)

## Account Credentials

### Finding Your Credentials

1. Log in to [Twilio Console](https://console.twilio.com/)
2. Your **Account SID** and **Auth Token** are displayed on the dashboard
3. Click the eye icon to reveal your Auth Token

### Environment Variables

```bash
export TWILIO_ACCOUNT_SID="ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export TWILIO_AUTH_TOKEN="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export TWILIO_PHONE_NUMBER="+15551234567"
```

---

## Phone Number Setup

### Purchasing a Phone Number

1. Go to **Phone Numbers** → **Manage** → **Buy a number**
2. Search by country, capabilities (Voice, SMS, MMS), or area code
3. Select a number and click **Buy**

### Checking Number Capabilities

1. Go to **Phone Numbers** → **Manage** → **Active numbers**
2. Click your phone number
3. Check the **Capabilities** section:
   - **Voice**: Required for phone calls
   - **SMS**: Required for text messaging
   - **MMS**: Required for media messages (images, video)

---

## Voice Configuration

### Configure Voice Webhooks

1. Go to **Phone Numbers** → **Manage** → **Active numbers**
2. Click your phone number
3. Scroll to **Voice Configuration**
4. Set the following:

| Setting | Value |
|---------|-------|
| **Configure with** | Webhooks, TwiML Bins, Functions, etc. |
| **A call comes in** | Webhook |
| **URL** | `https://your-server.com/voice/inbound` |
| **HTTP Method** | POST |
| **Status callback URL** | `https://your-server.com/voice/status` |

5. Click **Save configuration**

### Voice Webhook URLs

| Webhook | URL | Purpose |
|---------|-----|---------|
| Voice URL | `/voice/inbound` | Handles incoming calls |
| Status Callback | `/voice/status` | Receives call status updates |

### Starting the Voice Gateway

```bash
# With public URL
omniagent voice serve --provider twilio --public-url https://your-server.com

# With OpenAI Realtime (native voice-to-voice)
omniagent voice serve --provider twilio --realtime openai --public-url https://your-server.com

# With ngrok for local development
export NGROK_AUTHTOKEN=your-token
omniagent voice serve --provider twilio --ngrok
```

---

## SMS/MMS Configuration

### Configure SMS Webhooks

1. Go to **Phone Numbers** → **Manage** → **Active numbers**
2. Click your phone number
3. Scroll to **Messaging Configuration**
4. Set the following:

| Setting | Value |
|---------|-------|
| **Configure with** | Webhooks, TwiML Bins, Functions, etc. |
| **A message comes in** | Webhook |
| **URL** | `https://your-server.com/webhook/twilio/sms` |
| **HTTP Method** | POST |

5. Click **Save configuration**

### SMS Webhook URLs

| Webhook | URL | Purpose |
|---------|-----|---------|
| Message URL | `/webhook/twilio/sms` | Handles incoming SMS/MMS |

### Running OmniAgent with SMS

```bash
# Environment variables
export TWILIO_ACCOUNT_SID="AC..."
export TWILIO_AUTH_TOKEN="..."
export TWILIO_PHONE_NUMBER="+15551234567"

# Start the gateway
omniagent gateway run
```

---

## RCS Configuration (Rich Communication Services)

RCS provides rich messaging features with automatic fallback to SMS/MMS.

### Create a Messaging Service

1. Go to **Messaging** → **Services**
2. Click **Create Messaging Service**
3. Enter a friendly name (e.g., "OmniAgent RCS")
4. Click **Create Messaging Service**

### Add Your Phone Number to the Service

1. In your Messaging Service, go to **Sender Pool**
2. Click **Add Senders**
3. Select **Phone Number**
4. Choose your phone number
5. Click **Add Phone Numbers**

### Add RCS Sender (Requires Approval)

1. In your Messaging Service, go to **Sender Pool**
2. Click **Add Senders**
3. Select **RCS Sender**
4. Follow the RCS onboarding process (requires carrier approval)

### Configure Environment

```bash
export TWILIO_ACCOUNT_SID="AC..."
export TWILIO_AUTH_TOKEN="..."
export TWILIO_MESSAGING_SERVICE_SID="MGxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
```

!!! note "RCS Fallback"
    When `TWILIO_MESSAGING_SERVICE_SID` is set, messages are sent via RCS. If the recipient doesn't support RCS, Twilio automatically falls back to SMS/MMS.

---

## Local Development with ngrok

For local development, use ngrok to expose your local server to Twilio.

### Setup ngrok

1. Sign up at [ngrok.com](https://ngrok.com)
2. Get your auth token from the [dashboard](https://dashboard.ngrok.com/get-started/your-authtoken)
3. Set the environment variable:

```bash
export NGROK_AUTHTOKEN=your-ngrok-auth-token
```

### Using ngrok with Voice

```bash
# Start voice gateway with ngrok tunnel
omniagent voice serve --provider twilio --ngrok --listen :8081

# Output will show:
# Voice gateway starting...
# ngrok tunnel: https://abc123.ngrok.io
# Configure Twilio Voice URL: https://abc123.ngrok.io/voice/inbound
```

### Using ngrok with SMS

```bash
# Start gateway on a specific port
export OMNIAGENT_GATEWAY_ADDRESS=":8080"
omniagent gateway run

# In another terminal, start ngrok
ngrok http 8080

# Configure Twilio SMS webhook to:
# https://abc123.ngrok.io/webhook/twilio/sms
```

### Updating Twilio Webhooks

After starting ngrok, update your Twilio phone number webhooks:

1. Copy the ngrok HTTPS URL (e.g., `https://abc123.ngrok.io`)
2. Go to **Phone Numbers** → **Active numbers** → your number
3. Update the webhook URLs with the ngrok URL
4. Save configuration

!!! warning "ngrok URL Changes"
    Free ngrok URLs change each time you restart. For persistent URLs, use ngrok's paid plans or deploy to a server with a static URL.

---

## Combined Voice + SMS Setup

To run both voice and SMS on the same server:

### Environment Variables

```bash
# Twilio credentials
export TWILIO_ACCOUNT_SID="AC..."
export TWILIO_AUTH_TOKEN="..."
export TWILIO_PHONE_NUMBER="+15551234567"

# Optional: RCS
export TWILIO_MESSAGING_SERVICE_SID="MG..."

# LLM Provider
export ANTHROPIC_API_KEY="sk-ant-..."

# For OpenAI Realtime voice
export OPENAI_API_KEY="sk-..."
```

### Twilio Console Configuration

Configure your phone number with both Voice and Messaging webhooks:

| Section | Setting | URL |
|---------|---------|-----|
| **Voice** | A call comes in | `https://your-server.com/voice/inbound` |
| **Voice** | Status callback | `https://your-server.com/voice/status` |
| **Messaging** | A message comes in | `https://your-server.com/webhook/twilio/sms` |

### Starting the Server

```bash
# Option 1: Gateway only (SMS)
omniagent gateway run

# Option 2: Voice gateway (handles both voice and can mount SMS webhook)
omniagent voice serve --provider twilio --public-url https://your-server.com
```

---

## Troubleshooting

### Voice Calls Not Connecting

1. **Check webhook URL**: Ensure the Voice URL is publicly accessible
2. **Check TLS**: Twilio requires valid HTTPS certificates
3. **Check logs**: Look for errors in Twilio Console → Monitor → Logs
4. **Test webhook**: Use Twilio's "Make a test call" feature

### SMS Not Receiving

1. **Check webhook URL**: Ensure the Message URL is correct
2. **Check HTTP method**: Must be POST
3. **Check response**: Webhook must return 200 OK with TwiML or empty response
4. **Check phone format**: Use E.164 format (`+15551234567`)

### ngrok Tunnel Issues

1. **Auth token**: Ensure `NGROK_AUTHTOKEN` is set
2. **Port mismatch**: Ensure ngrok port matches server port
3. **URL expired**: Free ngrok URLs expire; restart and update Twilio webhooks

### Common Error Codes

| Code | Meaning | Solution |
|------|---------|----------|
| 11200 | HTTP retrieval failure | Check webhook URL is accessible |
| 11205 | HTTP connection failure | Check server is running |
| 12100 | Document parse failure | Check TwiML response format |
| 21211 | Invalid phone number | Use E.164 format |
| 21608 | Unverified phone number | Verify caller ID in Twilio Console |

### Twilio Debugger

1. Go to **Monitor** → **Logs** → **Errors**
2. Click on an error to see details
3. Check the Request and Response tabs for debugging

---

## Security Best Practices

### Validate Webhook Signatures

Twilio signs all webhook requests. Validate signatures in production:

```go
// The omni-twilio webhook handlers validate signatures automatically
// when AuthToken is configured
```

### Use Environment Variables

Never hardcode credentials:

```bash
# Good: Environment variables
export TWILIO_AUTH_TOKEN="..."

# Bad: Hardcoded in code
// authToken := "your-token-here"  // DON'T DO THIS
```

### Restrict Webhook Access

If possible, restrict webhook endpoints to Twilio's IP ranges. See [Twilio's IP addresses](https://www.twilio.com/docs/usage/webhooks/webhooks-security#validating-twilio-requests).

---

## Next Steps

- [Voice Integration](voice.md) - Detailed voice gateway configuration
- [LLM Providers](providers.md) - Configure AI providers
- [Environment Variables](../reference/environment.md) - Complete reference
