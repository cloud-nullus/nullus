# AI Assistant

<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>
<span class="role role-developer">developer</span>

Open it with the **AI assistant** button at the bottom right of every screen (above the shortcuts badge).
Ask about stacks, clusters, or install status in plain language; the server checks with the
[MCP](../reference/mcp.md) read tools and answers. The organization's admin picks the model — Upstage Solar · OpenAI · Anthropic Claude · Google Gemini · an
OpenAI-compatible in-house server.

!!! note "No button until an admin turns it on"
    An admin has to register a provider and API key and turn it on under **Admin › AI Assistant** (see
    [Turning it on](#turning-it-on)). It is per organization; while it is off, `GET /api/v1/assistant/status`
    returns `{"enabled": false}` and the screen does not draw the button.

## What you can ask

An empty conversation shows three suggested questions — **Are any stacks failing?** · **Why did the last
stack install fail?** · **Which stack templates are available?**

The model picks from the tools below and answers from their results. Each answer carries a
**Tools used** line, and a tool that failed is marked "(failed)".

| Tool | Used for |
|------|----------|
| `stack_list` | Stacks and their status |
| `stack_status` | Deploy progress · the failed step and its reason |
| `stack_logs_tail` | The last lines of the install log — why an install failed |
| `cluster_list` | Registered clusters (secrets such as kubeconfig removed) |
| `template_list` | The stack template catalog |
| `compat_check` | Compatibility of a tool combination |

## It sees only what the person asking can see

The tools call the API carrying the requester's own credentials. Whatever you cannot see on screen, the
assistant cannot see either — for example, a developer has no access to the stack and cluster APIs, so the
assistant answers that they lack permission ([Permissions](../reference/rbac.md)).

## It only reads

Change tools such as deploy and rollback are not in the assistant's tool list at all. When a change is
needed, it tells you where on screen to make it.

## Data that leaves the platform

!!! warning "Tool results are sent to an external model"
    While it is on, the results of the tools above (stack config · status · parts of install logs) go to the
    chosen provider. The panel says so at the bottom. If you do not want that, leave it off, or pick
    **OpenAI-compatible (in-house server)** and use a model running in-house (vLLM · Ollama, for example).

- Before sending, it masks `Bearer` tokens · JWTs · GitLab tokens (`glpat-`) · AWS access keys · private key
  blocks · values such as `password=` inside strings
- Conversations are not stored on the server. The screen keeps the conversation and sends all of it with each
  question, so closing and reopening the panel keeps it, and a reload clears it

## Turning it on

<span class="route">/admin/assistant</span>
<span class="role role-admin">admin</span>

Register the organization's provider, model, and API key under **Admin › AI Assistant**.

| Provider | Default model | Address |
|----------|---------------|---------|
| Upstage Solar | `solar-pro4` | `https://api.upstage.ai/v1` |
| OpenAI | Enter it | `https://api.openai.com/v1` |
| Anthropic Claude | `claude-sonnet-5-5` | `https://api.anthropic.com` |
| Google Gemini | Enter it | `https://generativelanguage.googleapis.com/v1beta/openai` |
| OpenAI-compatible (in-house server) | Enter it | Enter it — the key is optional |

1. Pick a provider, and enter a model name and address if needed (empty means the defaults above)
2. Enter the **API key**. After saving it is never shown again, only its last four characters — leave the
   field empty when saving to keep the saved key. Switching provider needs the new provider's key
3. **Test connection** asks the model once with those settings (nothing is saved)
4. Tick **Turn on the assistant for this organization** and click **Save**

The key is encrypted with the server's `ENCRYPTION_KEY` — without it, the key is not saved. Saving and
deleting are recorded in the audit log (never the key itself), and **Delete settings** removes the key too and
turns the assistant off.

## Limitations

- A conversation holds up to 20 turns, each up to 4,000 characters. One question can use up to 6 rounds of
  tool calls · 8 calls per round · 20 calls in all
- One question gets up to 55 seconds. Past that, the panel asks you to try a narrower question
- A tool result longer than 12,000 characters is cut, and only its beginning and end go to the model (an
  install log has the failure at the end)
- The answer appears all at once when it is ready (no streaming)
- Tool calls go to the API itself over loopback (127.0.0.1), so every user's tool calls share one IP ceiling
  (600 per minute by default), and they also count against the requester's own user rate limit

## APIs it calls

`GET /api/v1/assistant/status` · `POST /api/v1/assistant/chat` · admin:
`GET · PUT · DELETE /api/v1/admin/assistant` · `POST /api/v1/admin/assistant/test`
