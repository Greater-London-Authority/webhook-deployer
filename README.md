# Webook deployer

`webhook-deployer` is a tool that listens for a `workflow_run.completed` webhook triggered by GitHub Actions, downloads the associated build artifcat `.zip` file, and extacts its contents to a specified directory. 

## Motivating problem

We have a large numebr of web applications. When one of these is updated, we want to perform a build process (typically running `npm run build`), then copy the static files to a server, where they are served by a web-server such as Caddy/Apache/nginx.


### Alternative approaches

There are several alternative approaches:

* we could add a step to a GitHub workflow that copies build artifacts to a server using `scp` or `rsync` (e.g., see these [blog](https://rderik.com/blog/a-simple-setup-for-a-build-and-deploy-system-using-github-actions/#the-build-and-deploy-architecture) [posts](https://dev.to/koddr/automate-that-a-practical-guide-to-github-actions-build-deploy-a-static-11ty-website-to-remote-virtual-server-after-push-d19#ch-5))
This has the disadvantages of requiring server credentials to be stored on GitHub as [secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets), and for firewalls to allow SSH connections to be made form GitHub to the server; both of these are things that some administrators may prefer to avoid for security reasons.

* we could use a generic tool like [`webhook`](https://github.com/adnanh/webhook) that receives webhooks and repsosnds shell scripts in response (e.g., as described in [these](https://maximorlov.com/automated-deployments-from-github-with-webhook/) [blog](https://betterprogramming.pub/how-to-automatically-deploy-from-github-to-server-using-webhook-79f837dcc4f4) posts). This would make it easy to repsond to a `git push` by doing a `git pull` and build on the server, but this would require build tools to be installed on the server; however, downloading build artifacts produced by a GitHub Actions run is more difficult to do from a shell script as it requires first making an API request to get the URL of the zip file.

* rather than using GitHub Actions, we could run builds on a self-hosted CI/CD server on a trusted network, and copy build artifacts form there. This requires us to maintain addiitonal infrastructure; since the source code is already on GitHub it makes sense to make use of the integration with GitHub Actions.

* we could give up on automation entirely, and instead build code manually on developer machines, and copy build artifacts form there: this carries a risk of divergence between the source code that is in git and what is actually deployed

* we could give up on serving files from our own server, and instead have GitHub Actions deploy to S3/Vercel/Netlify/Cloudflare Pages. This would have a various advantages (such as supporting preview/testing deployments), but requires going through a process to approve spending on a new subscription service.


## Development

Run: `go run .`

Build: `go build .`


## Deployment

### Creating a GitHub token

In order to donload build artifacts, `webhook-deployer` needs to be provided a [fine-grained personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) with  `Read-only` access to the `Actions` and `Metadata` Repository Permissions scopes.


### Running webhook-deployer

Create a config file, using [`config.json`](./config.json) as a template.

You then need to run the service, passing the name/path of the config file as an argument is if is not `./config.json`.

You can redirect logging output to a file and detatch with:

    ./webhook-deployer webhook-config.json >> ./webhooks.log 2>&1

You may want to proxy requests with a server such as nginx.


### Configuring deployment for a repository

#### On GitHub

You will need to create a YAML file in the `.github/workflows` diectory of your repo. Here is an example


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

In the `Webhooks` section of the repo settings, you will also need to create a webhook:

* set the `Payload URL` to the appropriate URL (the IP address/domain name of your server and the port defined in the config file, or whever it is proxied if you are using a reverse proxy)
* set the `Secret` to a random value, and record the same value in the config file. This si sued to verify that webhook requests came from GitHub.
* select the `Let me select individuual events` option for `Which events would you like to trigger this webhook?`, and then selct `Workflow runs`.


#### In the webhook-deployer config

Add an entry to the `projects` key of the config file, that specified:

* `repository`: the full neame of the repository (e.g., *<org or account name>/<project name>*)
* `destination`: the path on the system where the contents of the artifact zip file should be extracted to
* `workflow_path`: the path within the repo containing the file defining the workflow that should trigger the deployment (e.g., `.github/workflows/build.ym`)


## Relevant documentation

* [Getting a personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token)
* [GitHub Actions Documentation](https://docs.github.com/en/actions)
* [Storing workflow data as artifacts](https://docs.github.com/en/actions/using-workflows/storing-workflow-data-as-artifacts)
* [GitHub Webhooks documentation](https://docs.github.com/en/webhooks-and-events/webhooks)
* [REST API to list workflow run artifacts](https://docs.github.com/en/rest/actions/artifacts?apiVersion=2022-11-28#list-workflow-run-artifacts)
