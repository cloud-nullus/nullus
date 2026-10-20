# AI Assistant

<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>
<span class="role role-developer">developer</span>

Open it with the **AI assistant** button at the bottom right of every screen (above the shortcuts badge).
Ask about stacks, clusters, or install status in plain language; the server checks with the
[MCP](../reference/mcp.md) read tools and answers. The default model is Upstage Solar.

!!! note "No key on the server, no button"
    It turns on only when `NULLUS_ASSISTANT_API_KEY` (chart: `secrets.assistantApiKey`) is set. Without a
    key, `GET /api/v1/assistant/status` returns `{"enabled": false}` and the screen does not draw the button.

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
    configured model (Upstage by default). The panel says so at the bottom. If you do not want that, leave
    the key unset, or point `NULLUS_ASSISTANT_BASE_URL` at an in-house OpenAI-compatible server.

- Before sending, it masks `Bearer` tokens · JWTs · GitLab tokens (`glpat-`) · AWS access keys · private key
  blocks · values such as `password=` inside strings
- Conversations are not stored on the server. The screen keeps the conversation and sends all of it with each
  question, so closing and reopening the panel keeps it, and a reload clears it

## Turning it on

| Environment variable | Chart value | Default |
|----------------------|-------------|---------|
| `NULLUS_ASSISTANT_API_KEY` | `secrets.assistantApiKey` | None — off |
| `NULLUS_ASSISTANT_BASE_URL` | `config.assistant.baseURL` | `https://api.upstage.ai/v1` |
| `NULLUS_ASSISTANT_MODEL` | `config.assistant.model` | `solar-pro4` |

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

`GET /api/v1/assistant/status` · `POST /api/v1/assistant/chat`
