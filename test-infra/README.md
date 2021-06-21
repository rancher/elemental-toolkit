## ECP github runners

This is a simple script + heat template to create workers on ECP with a github runner


### Requirements

 - Cloud credentials obtained from ECP(Openstack RC file). Make sure to load the creds into the shell by sourcing the file (File can be downloaded from Project -> API access)
 - Openstack python client with heat plugin
 - Runner token (Obtained in the repo settings -> Actions -> Add Runner)


### Usage

```bash
/manage-workers.sh -c --worker-name $NAME --token $TOKEN
```

Where token is the runner token, name is whatever you wanna call the heat stack and  -c means create


To remove the stack:

```bash
./manage-workers.sh -d --worker-name $NAME
```

There is a few extra options that override the default config that you can see by running the script with no params:

```bash
    --worker-flavor     The heat flavor to use for the workers (for the default see the heat template for the worker type)
    --worker-name       The name for the worker appended to the stack name
    --worker-template   Template used for creating the worker (github-runner) (default: github-runner)
    --image             Name of the worker image
    --token             Github runner token
    --github_url        Github url for runner token (default: https://github.com/rancher-sandbox/cOS-toolkit)
    --labels            Labels to add to the runner (comma separated list)

```


## Removing runners

When bringing down the runner on ECP we need to manually go into the repo settings and remove it from the list.
