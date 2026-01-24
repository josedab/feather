# Feather Discord Server Setup Guide

This guide describes how to set up and manage the official Feather community Discord server.

---

## Recommended Channel Structure

### Information
| Channel | Purpose |
|---------|---------|
| `#announcements` | Release notes, blog posts, and official project updates (read-only for members) |
| `#rules` | Server rules and code of conduct (read-only) |

### Community
| Channel | Purpose |
|---------|---------|
| `#general` | Open discussion about Feather, ML infrastructure, and feature stores |
| `#help` | Ask questions about installation, configuration, and usage |
| `#showcase` | Share what you've built with Feather — demos, integrations, benchmarks |

### Development
| Channel | Purpose |
|---------|---------|
| `#contributing` | Discuss contributions, PRs, and good first issues |
| `#features` | Propose, discuss, and refine new feature ideas |
| `#bug-reports` | Report and triage bugs (supplement to GitHub Issues) |

### Maintainers (private)
| Channel | Purpose |
|---------|---------|
| `#maintainers` | Private channel for core maintainers to coordinate releases and triage |
| `#ci-alerts` | Automated CI/CD failure notifications |

---

## Bot Recommendations

### GitHub Webhook Integration
Use the **GitHub → Discord webhook** to post notifications to relevant channels:

| Event | Channel |
|-------|---------|
| New issues / PRs | `#contributing` |
| Releases / tags | `#announcements` |
| CI failures | `#ci-alerts` (maintainers only) |
| Stars milestones (100, 500, 1k) | `#general` |

**Setup:**
1. In Discord, go to **Server Settings → Integrations → Webhooks**.
2. Create a webhook for each target channel.
3. In GitHub, go to **Settings → Webhooks → Add webhook**.
4. Set the Payload URL to the Discord webhook URL + `/github` suffix.
5. Select the events you want to forward.

### Recommended Bots
- **MEE6** or **Carl-bot** — Auto-role assignment, welcome messages, moderation.
- **Disboard** — List the server on Discord server directories for discoverability.

---

## Moderation Guidelines

1. **Be welcoming.** Every question is valid. Redirect, don't dismiss.
2. **Enforce the Code of Conduct.** Feather follows the [Contributor Covenant](https://www.contributor-covenant.org/). Zero tolerance for harassment, discrimination, or personal attacks.
3. **Warn → Mute → Ban.** Escalate proportionally. Document actions in `#maintainers`.
4. **No spam or self-promotion.** Off-topic promotions are removed. Relevant project showcases go in `#showcase`.
5. **Keep discussions constructive.** Technical disagreements are healthy; personal attacks are not.
6. **Protect privacy.** Never share private messages, emails, or personal information without consent.

### Moderator Roles
| Role | Permissions |
|------|------------|
| `@Admin` | Full server management (maintainers only) |
| `@Moderator` | Manage messages, mute/kick members, manage threads |
| `@Contributor` | Assigned after first merged PR — access to `#contributing` |
| `@Member` | Default role for verified server members |

---

## Welcome Message Template

Configure this as the auto-message in `#general` (via MEE6 or Carl-bot):

```
👋 Welcome to the Feather community, {user}!

Feather is a high-performance, real-time feature store written in Go.

Here's how to get started:
• 💬 Say hi in #general
• ❓ Ask questions in #help
• 🔧 Want to contribute? Check #contributing and our Good First Issues
• 🚀 Built something cool? Share it in #showcase

Useful links:
• GitHub: https://github.com/feather-store/feather
• Docs: https://feather.dev/docs
• Quick Start: `make build && make run-dev`

Please read our #rules and Code of Conduct. We're glad you're here! 🪶
```

---

## Quick Setup Checklist

- [ ] Create server with the channel structure above
- [ ] Set `#announcements` and `#rules` to read-only
- [ ] Configure roles (`@Admin`, `@Moderator`, `@Contributor`, `@Member`)
- [ ] Set up GitHub webhook integration
- [ ] Install moderation bot (MEE6 or Carl-bot)
- [ ] Configure welcome message
- [ ] Post Code of Conduct in `#rules`
- [ ] Invite core maintainers and assign `@Admin`
- [ ] List on Disboard for discoverability
- [ ] Add Discord invite link to README and docs
