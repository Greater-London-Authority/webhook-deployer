# Webook deployer

Webhook Deployer is a lightweight service written in Go that listens for GitHub webhooks (specifically, for GitHub Action workflow run completion and branch deletion events); it downloads the build artifacts uploaded by GitHub Actions, and deploys them to a local directory.
It can optionally send notifications via [ntfy.sh](https://ntfy.sh/) when a deployment succeeds.


## Overview

Webhook Deployer automates the deployment workflow for web applications. When a GitHub Actions workflow completes and its artifact (a ZIP file) is ready, the service performs the following steps:

1. Receives a `workflow_run` webhook as an HTTP POST request.
2. (Optionally) Validates the webhook signature to verify that it originated from GitHub.
3. Uses the [GitHub personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens) provided in the config file to download the generated ZIP artifact.
4. Deletes any previous content in the destination directory.
5. Extracts the contents of the ZIP file to the destination.
6. (Optionally) Updates a deployment log with the identifier of the commit that was deployed, and the time at which it was deployed.
7. (Optionally) Sends a notification via ntfy.sh.

It also listens for branch deletion webhooks; if branch previews were enabled for the correspoding repository, then it deletes the directory for the relevant branch preview.

This design offloads the build from the server to GitHub Actions (reducing load on the server and avoiding the need to install development tools on it), without storing SSH credentials for copying the built files to the server.


## Motivating problem

We have a large number of web applications. When one of these is updated, we want to perform a build process (typically running `npm run build`), then copy the static files to a server, where they are served by a web-server such as Caddy/Apache/nginx.

We want a process that:
- Performs automated isolated builds of the exact code that is checked out from Git, without further modifications.
- Avoids storing sensitive credentials such as SSH keys on GitHub (or any CI/CD server).
- Prevents the need for exposed SSH ports or additional build infrastructure on the deployment server.
- Centrally manages artifact downloads and deployments.


### Alternative approaches

There are several alternative approaches:

* we could add a step to a GitHub workflow that copies build artifacts to a server using `scp` or `rsync` (e.g., see these [blog](https://rderik.com/blog/a-simple-setup-for-a-build-and-deploy-system-using-github-actions/#the-build-and-deploy-architecture) [posts](https://dev.to/koddr/automate-that-a-practical-guide-to-github-actions-build-deploy-a-static-11ty-website-to-remote-virtual-server-after-push-d19#ch-5))
This has the disadvantages of requiring server credentials to be stored on GitHub as [secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets), and for firewalls to allow SSH connections to be made form GitHub to the server; for security reasons, we would prefer to avoid both.

* we could use a generic tool like [`webhook`](https://github.com/adnanh/webhook) that receives webhooks and runs shell scripts in response (e.g., as described in [these](https://maximorlov.com/automated-deployments-from-github-with-webhook/) [blog](https://betterprogramming.pub/how-to-automatically-deploy-from-github-to-server-using-webhook-79f837dcc4f4) posts). This would make it easy to repsond to a `git push` by doing a `git pull` and build on the server, but this would require build tools to be installed on the server, which we would prefer to avoid. Downloading build artifacts produced by a GitHub Actions run is more difficult to do from a shell script as it requires a two-step process of making an API request to get the URL of the zip file, and then making a request for this file.

* we could give up on GitHub Actions, and run builds on a self-hosted CI/CD server on a trusted network, and copy build artifacts from there. This requires us to maintain addiitonal infrastructure; since the source code is already on GitHub it makes sense to make use of the integration with GitHub Actions.

* we could give up on automation entirely, and instead build code manually on developer machines, and copy build artifacts from there: this carries a risk of divergence between the source code that is in git and what is actually deployed

* we could give up on serving files from our own server, and instead have GitHub Actions deploy to S3/Vercel/Netlify/Cloudflare Pages. This would have a various advantages (such as supporting preview/testing deployments), but has a cost and is another subscription service to manage.


## Configuration File format

The service uses a JSON configuration file to set its parameters. A sample configuration (`config.json`) looks like this:

```json
{
  "listen": ":8080",
  "secret": "THIS_IS_A_SECRET",
  "GH_TOKEN": "THIS_IS_ALSO_A_SECRET!",
  "deploy_log": "./deployments.json",
  "projects": [
    {
      "repository": "Greater-London-Authority/app-ev-charger-dashboard",
      "destination": "/var/www/html/ev-charger-dashboard",
      "workflow_path": ".github/workflows/build.yml",
      "ntfy_topic": "my-app-deployments",
      "allow_branch_previews": true
    }
  ]
}
```

### Configuration Fields

- **listen** (string):
  The network interface and port on which the service listens (default is `":8080"`).

- **secret** (string):
  (Informational in the config file) The secret used by GitHub to sign webhook payloads.
  **Note:** The service currently reads the secret from the `GITHUB_SECRET` environment variable to validate the signature.

- **GH_TOKEN** (string):
  A fine-grained personal access token with read-only permissions for GitHub Actions and repository metadata. This token is required to download build artifacts.

- **deploy_log** (string):
  The file path where the deployment log (a JSON file recording commit hashes and deployment timestamps) is stored.

- **projects** (array):
  An array of project objects. Each object defines how a particular repository should be deployed:
  - **repository** (string): The full repository name (e.g., `"Owner/Repo"`).
  - **destination** (string): The local directory where the artifact will be extracted.
  - **workflow_path** (string): The path within the repo to the workflow file that triggers deployment.
  - **ntfy_topic** (string): (Optional) A single ntfy.sh topic name for notifications.
  - **ntfy_topics** (array of strings): (Optional) An alternative to `ntfy_topic`, specify multiple topics.
  - **allow_branch_previews** (boolean): If `true`, then deployments for branches other than `master` or `main` will be deployed to a modified destination directory with a `-<branch_name>` suffix. These branch previews will be deleted when the corresponding branch deletion webhook is reveived.


> [!IMPORTANT]  
> The `destination` of each project must be unique. If they are not, then the service assumes there is a mistake in the config and refuses to start.


## Configuring deployment

### On GitHub

#### Workflow File Example

Create a YAML file in your repository's `.github/workflows` directory (for example: `build-and-deploy.yml`) to build the project and upload the build artifact.

```yaml
name: Build and upload build artifact

on:
  workflow_dispatch:
  push:
    branches:
      - main

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2

    - uses: actions/setup-node@v3
      with:
        node-version: 18
        cache: npm

    - name: Install dependencies
      run: npm ci

    - name: Build
      run: npm run build

    - name: Upload build artifact
      uses: actions/upload-artifact@v2
      with:
        name: build-artifact
        path: build
```

This workflow runs whenever there is a push to the `main` branch (or it is triggered by a manual `workflow_dispatch`), and builds the project then uploads the `build` directory as an artifact.


#### Configuring the GitHub Webhook

1. **Navigate to Repository Settings:**
   Go to your repository on GitHub, then click on **Settings** > **Webhooks**.

2. **Create a New Webhook:**
   - **Payload URL:** Set this to `http://<your-server-domain-or-ip>:8080/` (adjust the port if needed or if behind a reverse proxy).
   - **Content type:** Choose `application/json`.
   - **Secret:** Provide a random secret. Make sure to set the environment variable `GITHUB_SECRET` on your deployment server to this same value so that incoming payloads can be validated.
   - **Which events would you like to trigger this webhook?**
     Select **Let me select individual events** and then choose:
     - **Workflow runs** (for deployments)
     - **Delete** (for branch deletion events, if you use branch previews)

3. **Save the Webhook.**

When a workflow run completes or a branch is deleted, GitHub will send a POST request to your server. Webhook Deployer will then process the event according to your configuration.

Alternatively, you can usie the Organization settings to define webhooks that will run for all repositories in that organization.



#### In the webhook-deployer config

Add a new entry to the `projects` key of the config file, with the format described above.


#### Creating a GitHub Personal Access Token

In order to download build artifacts, `webhook-deployer` needs to be provided a [fine-grained personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) with  `Read-only` access to the `Actions` and `Metadata` Repository Permissions scopes.
You should do this once, rather than individually for each project.


## Notifications with ntfy.sh

If you have configured either `ntfy_topic` or `ntfy_topics` for a project in your config file, the service will send a notification to the specified ntfy.sh topic upon successful deployment.
The notification usually contains:
- A title (e.g., "Successful deployment")
- A message summarizing the deployment (the workflow path, repo name, and destination)
- An clickable link to view the corresponding GitHub Actions workflow run

No additional setup is required on your end; just ensure that your server can reach the ntfy.sh service, and subscribe to the relevant topic in a ntfy client (or the [ntfy web app](https://ntfy.sh/app)).

> [!IMPORTANT]  
> Unless you pay for ntfy pro and reserve a topic name, anyone who guess the topic name is able to view the messages sent to it.


## Deployment Log

The service optionally maintains a deployment log in the file specified by the **deploy_log** configuration field. This log is a JSON file containing, for each project, the last deployed commit hash and the timestamp of deployment. This information can help keep track of exactly which version of an application is deployed.


## Relevant documentation

* [Getting a personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token)
* [GitHub Actions Documentation](https://docs.github.com/en/actions)
* [Storing workflow data as artifacts](https://docs.github.com/en/actions/using-workflows/storing-workflow-data-as-artifacts)
* [GitHub Webhooks documentation](https://docs.github.com/en/webhooks-and-events/webhooks)
* [REST API to list workflow run artifacts](https://docs.github.com/en/rest/actions/artifacts?apiVersion=2022-11-28#list-workflow-run-artifacts)



## Development

### Prerequisites

- You will need to [install Go](https://golang.org/dl/)

### Running in Development

Clone the repository and run the service directly:

    go run .

Or specify a custom configuration file:

    go run . custom-config.json


### Building the Binary

To build the executable:

    go build .

You can then run the generated binary executable:

    ./webhook-deployer config.json

If you want the service to run in the background and log to a file, use:

    nohup ./webhook-deployer config.json >> webhook-deployer.log 2>&1 &

