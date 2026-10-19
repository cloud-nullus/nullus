# CI/CD Template

<span class="route">/cicd/templates</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

The screen where you pick a base pipeline template.

![Pipeline template list](../../assets/screens/cicd-templates-light.png#only-light)
![Pipeline template list](../../assets/screens/cicd-templates-dark.png#only-dark)

## How to use it

1. Browse the cards, or narrow them with search (matches name and description).
2. Click **Use Base Template** on a card.
3. You land on the [App Deploy](app-deploy.md) screen
   (`/cicd/developer-deploy?template=…`) with values prefilled.

!!! note "It does not go to `/cicd/create`"
    A separate pipeline creation screen (`/cicd/create`) exists, but this button
    does not go there. The details are in
    [CI/CD List](list.md#pipeline-creation-screen).

## Card layout

Templates are shown in three **CI/CD Type** groups — `Default` (Deployment ·
Service · Ingress) · `Helm` (Helm chart release) · `Cronjob/Job`. The screen
assigns the group itself by looking at the ID, name, and stages strings.

Each card shows the name, description, CI/CD Type, Application Type,
**Capabilities** badges (CI · CD · Test · Security), and the creator.

The two Nullus sample app templates are always sorted first.

!!! warning "The list differs from the server response"
    The screen changes two things.

    - It **removes** `web-frontend-v1` from the list
    - It shows `web-backend-v1` renamed to **`User Custom Pipeline`**

    The description text also comes from a Korean/English table hard-coded in the
    screen — not from the server's `description`.

## Create, edit, delete (admin)

**Create Template · Edit · Delete** are visible only to admin.

The fields are Template ID (locked when editing) · Name · Description ·
Application Type (`web` · `backend` · `batch`) · Capabilities checkboxes
(CI · CD · Test · Security). Name is required, and you must select at least one
capability.

!!! warning "Capabilities are saved into the stages column"
    On save, the dialog writes the checked Capability labels as-is into the
    server's `stages` column. So a template that admin creates has **values that
    are not real stage names**, such as `stages = ["CI","CD"]`. Those values are
    later drawn as the stage list in the history tab of [CI/CD List](list.md), so
    the stages can look odd.

## APIs it calls

| Action | Endpoint |
|------|-----------|
| List | `GET /api/v1/cicd/templates` |
| Create | `POST /api/v1/cicd/templates` |
| Update | `PUT /api/v1/cicd/templates/{id}` (fields you don't send keep their existing values) |
| Delete | `DELETE /api/v1/cicd/templates/{id}` |

!!! warning "developer cannot get here from the menu"
    The route allows developer, but there is no sidebar entry. A developer can
    get in through the start button on the home screen, but once they leave, the
    menu offers no way back.
