# Step-by-Step Guide: Deploy Latest Branch on Netlify

## Step 1: Log in to Netlify
Log in to Netlify at:
https://app.netlify.com/

Use the **GitHub SSO** method to sign in.
After logging in, navigate to the project dashboard at:
https://app.netlify.com/sites/kubernetes-sigs-cluster-api-ibmcloud

If you do not have access to this Netlify project, you will need to **request membership**.
Raise a Netlify project membership request similar to the following example:
https://github.com/kubernetes/org/issues/5284

> **Note:**
> - As of today, **only [@Prajyot-Parab](https://github.com/Prajyot-Parab)** has direct access to this specific Netlify project.
> - Members of the **Kubernetes Docs Team** in Netlify have access to **all Netlify projects under the Kubernetes organization**.

---

## Step 2: Open Your Project
From the Netlify dashboard, select the project (site) you want to deploy.

---

## Step 3: Go to Project Configuration
In your project dashboard, navigate to **Project configuration** (or **Site settings**).

---

## Step 4: Open Build & Deploy Settings
From the left-hand menu, click **Build & deploy**.

---

## Step 5: Configure Deploy Contexts
Scroll to the **Deploy contexts** section.

Add the **new release branch** to the list of branches that should trigger deployments.

---

## Step 6: Create a Pull Request to the Release Branch
In your Git repository (GitHub / GitLab / Bitbucket):

- Create a pull request targeting the **release branch**
- Ensure all checks pass
- Merge the pull request into the release branch

---

## Step 7: Trigger Netlify Branch Deployment
Once the pull request is merged, Netlify will automatically trigger a deploy for the **release branch** with the latest changes.

---

## Step 8: Verify Deployment in Netlify
In Netlify, navigate to the **Deploys** section of your project.

You can:
- **Review the branch deploy status and logs for the latest release version**
- Preview the deployed site
- Click **Retry deploy → Deploy with latest commit** if needed

---

## Step 9: Disable Branch Deploys for Custom Domains
In **Site settings → Domain management**, configure automatic deploy subdomains:

1. Go to **Automatic deploy subdomains**
2. Click **Edit custom domains**
3. **Uncheck “Branch deploys”**
4. Click **Save**

This ensures branch deploys do not automatically attach to your custom domain.

---

## Step 10: Configure Branch Subdomains
Still in **Site settings**, configure branch subdomains:

1. Go to **Domain management**
2. Navigate to **Branch subdomains**
3. Click **Add new branch subdomain**
4. Select or enter the **new release branch**
5. Click **Create branch subdomain**

---

## Step 11: Enable Branch Deploys for Custom Domains
In **Site settings → Domain management**, configure automatic deploy subdomains:

1. Go to **Automatic deploy subdomains**
2. Click **Edit custom domains**
3. **Check “Branch deploys”**
4. Click **Save**
