# Git Workflow

This document describes the Git workflow for the Tissi-Mah project. All team members must follow these guidelines to ensure a consistent and reliable development process.

## Table of Contents

1. [Branch Strategy](#branch-strategy)
2. [Development Flow](#development-flow)
3. [Hotfix Flow](#hotfix-flow)
4. [Merge Rules](#merge-rules)
5. [Branch Protection](#branch-protection)
6. [CI/CD Integration](#cicd-integration)
7. [Best Practices](#best-practices)

---

## Branch Strategy

### Main Branches

We use three long-lived branches, each corresponding to an environment:

| Branch | Environment | Deployment | Purpose |
|--------|-------------|------------|---------|
| `main` | Production | Manual | Stable, production-ready code |
| `staging` | Staging (AWS EKS) | Manual | Pre-production testing |
| `develop` | VPS-Dev (K3s) | Automatic | Integration and development testing |

```
main        ●─────────────────────────────●─────────── (production)
             ↑                             ↑
staging     ●───────────●─────────────────●─────────── (staging)
                         ↑                 ↑
develop     ●───●───●───●───●───●───●───●───●───●───── (vps-dev)
                ↑       ↑       ↑
features    ●───●   ●───●   ●───●
```

### Feature Branches

All development work happens in feature branches:

```
<type>/<description-courte>
```

**Types:**
- `feature/` — New functionality
- `fix/` — Bug fixes
- `hotfix/` — Urgent production fixes
- `refactor/` — Code refactoring
- `docs/` — Documentation only
- `test/` — Adding/modifying tests

**Examples:**
```
feature/user-authentication
feature/recurring-trips
fix/refund-calculation
refactor/booking-service-cleanup
docs/api-documentation
test/payment-integration-tests
```

---

## Development Flow

### Standard Feature Development

```
1. Create branch    2. Develop       3. PR to develop   4. PR to staging   5. PR to main
       │                 │                  │                  │                 │
       ▼                 ▼                  ▼                  ▼                 ▼
   feature/*  ──────► commits ──────► develop ──────────► staging ──────────► main
                                          │                  │                 │
                                          ▼                  ▼                 ▼
                                      vps-dev            staging           production
                                     (automatic)         (manual)          (manual)
```

### Step-by-Step Process

#### 1. Create a Feature Branch

```bash
# Start from develop
git checkout develop
git pull origin develop

# Create feature branch
git checkout -b feature/user-authentication
```

#### 2. Develop and Commit

```bash
# Make changes and commit following conventional commits
git add .
git commit -m "feat(auth): add Firebase token validation"

# Push to remote
git push origin feature/user-authentication
```

#### 3. Create Pull Request to develop

- Go to GitHub and create a PR: `feature/user-authentication` → `develop`
- Fill out the PR template completely
- Request review from 2 team members
- Wait for CI checks to pass
- Address review comments
- Once approved, merge using **Squash and merge**

#### 4. Promote to staging

After testing on vps-dev:

```bash
# Create PR from develop to staging
git checkout staging
git pull origin staging
```

- Create PR: `develop` → `staging`
- Get 2 approvals
- Merge using **Merge commit** (preserve history)
- Manually trigger staging deployment

#### 5. Release to production

After testing on staging:

- Create PR: `staging` → `main`
- Get 2 approvals
- Merge using **Merge commit**
- Manually trigger production deployment
- Create a release tag:

```bash
git checkout main
git pull origin main
git tag -a v1.2.0 -m "Release v1.2.0: User authentication feature"
git push origin v1.2.0
```

---

## Hotfix Flow

For urgent production bugs, hotfixes still follow the standard flow but with higher priority:

```
hotfix/*  ──────► develop ──────► staging ──────► main
                     │               │              │
                     ▼               ▼              ▼
                 vps-dev          staging       production
                (automatic)       (manual)       (manual)
```

### Hotfix Process

#### 1. Create Hotfix Branch

```bash
# Start from develop (always)
git checkout develop
git pull origin develop
git checkout -b hotfix/payment-gateway-crash
```

#### 2. Fix and Test

```bash
# Make the fix
git add .
git commit -m "fix(payment): handle null response from CinetPay"
git push origin hotfix/payment-gateway-crash
```

#### 3. Fast-Track Through Environments

- PR to `develop` → Get 2 approvals → Merge → Automatic deploy to vps-dev
- PR to `staging` → Get 2 approvals → Merge → Manual deploy to staging → Quick test
- PR to `main` → Get 2 approvals → Merge → Manual deploy to production

**Note:** Hotfixes should be small and focused. Mark PRs with `[HOTFIX]` prefix in the title for visibility.

---

## Merge Rules

### Approval Requirements

| Target Branch | Required Approvals | Who Can Approve |
|---------------|-------------------|-----------------|
| `develop` | 2 | Any team member (except author) |
| `staging` | 2 | Any team member (except author) |
| `main` | 2 | Any team member (except author) |

### Merge Methods

| Merge Type | When to Use |
|------------|-------------|
| **Squash and merge** | Feature branches → `develop` (clean history) |
| **Merge commit** | `develop` → `staging` and `staging` → `main` (preserve history) |
| **Rebase** | Never use on shared branches |

---

## Branch Protection

All main branches (`main`, `staging`, `develop`) are protected with these rules:

### Required Checks

| Rule | Description |
|------|-------------|
| **CI must pass** | All tests must pass before merging |
| **Code linted** | `golangci-lint` must pass with no errors |
| **Branch up-to-date** | Branch must be rebased on target before merge |
| **Test coverage ≥ 80%** | Minimum 80% code coverage required |
| **No TODO/FIXME** | No TODO or FIXME comments allowed in merged code |
| **Secret scan** | Automatic scan for hardcoded secrets, API keys, passwords |

### Review Requirements

| Rule | Description |
|------|-------------|
| **2 approvals required** | Minimum 2 team members must approve |
| **Author cannot approve** | PR author cannot approve their own PR |
| **Conversations resolved** | All review comments must be resolved |

### Stack-Specific Checks

| Rule | Description |
|------|-------------|
| **Proto compiled** | If `.proto` files modified, generated code must be up-to-date |
| **Migrations reversible** | Every `up` migration must have a corresponding `down` migration |
| **Helm values consistent** | New config must be added to all environment values files |

---

## CI/CD Integration

### Pipeline Triggers

| Event | Action |
|-------|--------|
| Push to any branch | Run tests, lint, coverage check |
| PR created/updated | Run full CI pipeline + security scan |
| Merge to `develop` | Run CI + **automatic deploy to vps-dev** |
| Merge to `staging` | Run CI (deploy is manual) |
| Merge to `main` | Run CI (deploy is manual) |

### Pipeline Stages

```yaml
stages:
  - lint          # golangci-lint, proto lint
  - test          # Unit tests, integration tests
  - coverage      # Check coverage ≥ 80%
  - security      # Secret scan, vulnerability scan
  - build         # Build Docker images
  - deploy        # Deploy to environment (if applicable)
```

### Deployment Commands

```bash
# Deploy to vps-dev (automatic, but can be triggered manually)
make deploy-vps-dev

# Deploy to staging (manual)
make deploy-staging

# Deploy to production (manual)
make deploy-prod
```

---

## Best Practices

### Commit Hygiene

**Do:**
- Write clear, descriptive commit messages using conventional commits
- Make small, focused commits (one logical change per commit)
- Commit often, push regularly

**Don't:**
- Commit commented-out code
- Commit TODO/FIXME (create an issue instead)
- Commit secrets or credentials
- Make giant commits with multiple unrelated changes

### Branch Hygiene

**Do:**
- Keep feature branches short-lived (max 1-2 weeks)
- Rebase on `develop` regularly to avoid conflicts
- Delete feature branches after merge

**Don't:**
- Work directly on `main`, `staging`, or `develop`
- Let branches become stale
- Merge without rebasing first

```bash
# Keep your branch up-to-date
git checkout feature/my-feature
git fetch origin
git rebase origin/develop

# If conflicts, resolve them then:
git rebase --continue
git push --force-with-lease
```

### Pull Request Hygiene

**Do:**
- Fill out the PR template completely
- Add meaningful description and testing instructions
- Respond to all review comments
- Keep PRs focused on one feature/fix

**Don't:**
- Create PRs without testing locally
- Ignore review comments
- Merge with unresolved conversations
- Mix multiple features in one PR

### Code Review Hygiene

**Do:**
- Review PRs within 24 hours
- Be constructive and specific in feedback
- Test the code locally if needed
- Approve only when you're confident

**Don't:**
- Approve without actually reviewing
- Be vague ("this looks wrong")
- Block PRs for style preferences (use linters instead)

---

## Quick Reference

### Common Commands

```bash
# Start a new feature
git checkout develop && git pull
git checkout -b feature/my-feature

# Keep branch updated
git fetch origin
git rebase origin/develop

# Push changes
git push origin feature/my-feature

# After merge, clean up
git checkout develop && git pull
git branch -d feature/my-feature
```

### Branch Naming Cheatsheet

| Type | Pattern | Example |
|------|---------|---------|
| Feature | `feature/<description>` | `feature/user-authentication` |
| Bug fix | `fix/<description>` | `fix/refund-calculation` |
| Hotfix | `hotfix/<description>` | `hotfix/payment-crash` |
| Refactor | `refactor/<description>` | `refactor/booking-cleanup` |
| Docs | `docs/<description>` | `docs/api-documentation` |
| Tests | `test/<description>` | `test/payment-tests` |

### Commit Message Cheatsheet

```
feat(scope): add new feature
fix(scope): fix bug
refactor(scope): refactor code
docs(scope): update documentation
test(scope): add tests
chore(scope): maintenance tasks
perf(scope): performance improvement
style(scope): formatting only
```

### Merge Flow Cheatsheet

```
feature/* ──► develop ──► staging ──► main
              (auto)      (manual)   (manual)
               
Approvals:      2            2          2
Merge type:   Squash      Merge      Merge
```