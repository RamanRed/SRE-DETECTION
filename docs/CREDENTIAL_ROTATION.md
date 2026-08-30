# Credential rotation and connection setup

Never paste credentials into source files, tickets, chat, build parameters, or
command-line arguments. If a credential is disclosed in any of those places,
treat it as compromised even when the repository is private.

## Immediate response to a disclosure

1. Revoke or deactivate the disclosed credential at its provider.
2. Review provider audit logs for use after the disclosure time.
3. Create a replacement with the minimum scope and a short expiration.
4. Store the replacement in the provider's secret store (Jenkins Credentials,
   GitHub Actions Secrets, Kubernetes Secret, or a cloud secret manager).
5. Re-run the secret scanner before deploying.

Do not merely delete the value from the latest Git commit. Rotation is required
because copies may remain in chat history, logs, caches, and repository history.

### Known repository-history finding

A GitHub personal-access-token-shaped value exists in the repository history
that predates the Go migration. It is absent from the current working tree, but
this branch does not rewrite shared Git history. Treat that token as compromised
and do not release or deploy until its revocation is confirmed. After rotation,
coordinate a repository-history cleanup with every contributor because rewriting
published history requires fresh clones and force-updated protected branches.

## Provider-specific replacements

### AWS

Deactivate the disclosed IAM access key immediately, inspect CloudTrail, and
then delete the key. Prefer an EC2 instance role, GitHub OIDC, or AWS IAM
Identity Center temporary credentials over another long-lived administrator
key. Terraform automatically uses the standard AWS provider credential chain;
no AWS credential belongs in a .tfvars file.

Official guidance:

- [Secure AWS access keys](https://docs.aws.amazon.com/IAM/latest/UserGuide/securing_access-keys.html)
- [Manage and delete IAM user access keys](https://docs.aws.amazon.com/IAM/latest/UserGuide/access-keys-admin-managed.html)

### GitHub repository access

Revoke the disclosed token and create a fine-grained token restricted to the
single repository and an expiration date. For repository polling and
code-aware triage, grant read-only repository metadata and contents. Add
Actions read/write only when this platform must dispatch a GitHub Actions
workflow. Add pull-request read only when pull-request context is required.

Store CI tokens in Jenkins Credentials, not in the Jenkinsfile. The connection
hub accepts a token as a write-only field; API responses contain only a
repositoryTokenConfigured boolean.

- [Managing GitHub personal access tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)

### Docker Hub

Deactivate/delete the disclosed personal access token and issue a CI-specific
replacement. A pipeline that publishes images needs read/write, not delete.
Store it as the Jenkins username/password credential referenced by the
Jenkinsfile. The pipeline uses a temporary Docker configuration directory and
removes it after the push.

- [Docker personal access tokens](https://docs.docker.com/security/access-tokens/)

### SonarQube Cloud

Revoke the disclosed token under **My Account > Security**. Prefer a scoped
organization token where the subscription supports one; otherwise create an
expiring user token whose account has only Execute Analysis access for this
project. Store it in the `sonarqube-token` Jenkins username/password
credential used by the Jenkinsfile; the username is ignored and the token
belongs in the password field.

- [Managing SonarQube Cloud tokens](https://docs.sonarsource.com/sonarqube-cloud/managing-your-account/managing-tokens)

### Jenkins

Change any disclosed login password, revoke the disclosed API token, and create
a dedicated service user instead of using the administrator account. Grant
only the permissions needed by the selected job: Job/Read, Job/Build, and
Job/Workspace. Generate a new API token from that service user's Security page
and save it in Jenkins Credentials.

- [Authenticating Jenkins scripted clients](https://www.jenkins.io/doc/book/system-administration/authenticating-scripted-clients/)
- [Using the Jenkins Credentials store](https://www.jenkins.io/doc/book/using/using-credentials/)

## Local and Kubernetes secrets

Use .env.example only as a list of variable names. Put local values in .env,
which is ignored by Git and excluded from Docker build contexts. In Kubernetes,
create Secrets out-of-band before applying the checked-in manifests. Do not
commit generated Secret YAML.

The integration API never returns repository or CI tokens. Production
deployments must configure four independent secret values through secret
references: the integration encryption key, session-signing secret, bootstrap
password, and CI webhook token. The signing secret must be at least 32 bytes
and the bootstrap password at least 12 bytes. See `.env.example` and
`k8s/incident-service.yml` for the current variable names.
