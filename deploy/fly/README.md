# Hosted Demo Deployment (Fly.io)

This directory contains configuration for deploying a read-only Feather demo instance on [Fly.io](https://fly.io).

## Prerequisites

- [flyctl](https://fly.io/docs/hands-on/install-flyctl/) installed
- Fly.io account (free tier works)

## Deploy

```bash
cd deploy/fly
fly launch --no-deploy    # Create the app (first time only)
fly deploy                # Deploy
fly status                # Check status
```

## Configuration

The demo runs with:
- **Read-only mode**: Write endpoints return 403
- **Rate limiting**: 100 requests/minute per IP
- **Sample data**: Pre-loaded with fraud detection feature set
- **Auto-scaling**: Min 0, max 2 instances (scales to zero when idle)

## Endpoints

Once deployed, the demo is available at:
- API: `https://feather-demo.fly.dev/health`
- Playground: `https://feather-demo.fly.dev/docs`
- OpenAPI: `https://feather-demo.fly.dev/v1/openapi.json`

## Cost

With Fly.io free tier + scale-to-zero:
- **Idle**: $0/month
- **Active**: ~$3-5/month (shared-cpu-1x, 256MB)
