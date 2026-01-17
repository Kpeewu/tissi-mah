# Pull Request Guidelines

This document explains how to create effective Pull Requests for the Tissi-Mah project. Following these guidelines ensures smooth code reviews, maintains project history, and helps onboard new team members.

## Why We Use PR Templates

When working on a microservices architecture with multiple developers, clear communication becomes critical. Our PR template solves several problems:

**For Authors:**
- Guides you through documenting your changes completely
- Reminds you of quality checks before requesting review
- Reduces back-and-forth questions from reviewers

**For Reviewers:**
- Provides context without needing to ask questions
- Clear testing instructions to validate changes
- Identifies which services and features are impacted

**For Future Reference:**
- Creates a searchable history of why changes were made
- Helps debug issues by tracing when features were introduced
- Onboards new developers by showing project evolution

## PR Template Sections

### Description

Write a clear, concise explanation of **what** your PR does and **why**. Avoid vague descriptions like "Fix bug" or "Update code."

**Bad Example:**
> Fix booking issue

**Good Example:**
> Fix incorrect price calculation when a passenger cancels a multi-segment booking. The refund was previously calculated on the total trip price instead of only the cancelled segments.

### Type of Change

Select the category that best describes your PR:

| Type | When to Use | Example |
|------|-------------|---------|
| ✨ Feature (`feat`) | New functionality for users or the system | Add recurring trip creation endpoint |
| 🐛 Fix (`fix`) | Fixing incorrect behavior | Fix wrong refund percentage calculation |
| 🚑 Hotfix (`hotfix`) | Urgent fix for production issues | Fix crash when payment provider returns null |
| ♻️ Refactor (`refactor`) | Improving code without changing behavior | Extract payment validation into separate function |
| 📚 Docs (`docs`) | Updating documentation only | Add API documentation for booking endpoints |
| 🎨 Style (`style`) | Formatting, no code logic change | Fix indentation, remove unused imports |
| ✅ Test (`test`) | Adding or updating tests only | Add unit tests for rating service |
| 🔧 Chore (`chore`) | Dependencies, config, CI/CD, Helm | Update Dockerfile, upgrade Firebase SDK |
| ⚡ Perf (`perf`) | Performance improvements | Optimize database query for trip search |
| 🏗️ Infrastructure | Terraform, K8s config changes | Add Redis cluster Terraform module |

### Impacted Service(s)

Check **all** services affected by your changes. This helps reviewers understand the scope and identify potential integration issues.

Our services:
- **auth-service** - Authentication, Firebase JWT validation
- **user-service** - User profiles, driver/passenger management
- **trips-service** - Trip creation, recurring patterns, waypoints
- **booking-service** - Reservations, segments, cancellations
- **payment-service** - Payments, refunds, payouts
- **rating-service** - User ratings and reviews
- **vehicle-service** - Vehicle management, documents
- **file-service** - Document storage and verification
- **notification-service** - Push notifications, emails, SMS
- **stats-service** - Analytics and reporting
- **geolocation-service** - Location tracking, routing

### Related Feature / Module

Specify which business feature your change affects. This adds context beyond just the service name.

**Examples:**
- "User registration flow"
- "Trip booking with intermediate stops"
- "Driver payout calculation"
- "Document verification process"

### How to Test

Provide **step-by-step** instructions that anyone can follow to verify your changes work correctly.

**Bad Example:**
> Test the booking endpoint

**Good Example:**
> 1. Start the local environment: `docker-compose up -d`
> 2. Create a test trip using the fixture: `make seed-trips`
> 3. Call the booking endpoint:
>    ```bash
>    grpcurl -plaintext -d '{"trip_id": "trip_123", "passenger_id": "user_456"}' \
>      localhost:50051 booking.BookingService/CreateBooking
>    ```
> 4. Verify the response contains `booking_reference` and `status: "pending"`
> 5. Check the database: `SELECT * FROM bookings WHERE trip_id = 'trip_123';`

### Expected Result

Describe what should happen when the test is successful. This helps reviewers confirm they're seeing correct behavior.

### Checklist

Go through each item before marking your PR as ready for review:

**Code Quality:**
- Run `make lint` to check for linting errors
- Run `make fmt` to format your code
- Search your code for hardcoded secrets or credentials

**Testing:**
- Run `make test` to verify all tests pass
- Test your changes manually using the steps you documented
- Add new tests if you've added new functionality

**Documentation:**
- Update relevant documentation if behavior changed
- Document new environment variables in the service README
- Update API documentation if endpoints changed

**Database & API:**
- Include migration files if you changed the database schema
- Run `make generate-proto` if you modified `.proto` files
- Ensure backward compatibility or document breaking changes

**Deployment:**
- Update Helm values files if you added new configuration
- Test that the CI pipeline passes on your branch

## Branch Naming Convention

Follow this pattern for branch names:

```
<type>/<description-courte>
```

**Types:**
- `feature/` - New functionality
- `fix/` - Bug fixes
- `hotfix/` - Urgent production fixes
- `refactor/` - Code refactoring
- `docs/` - Documentation only
- `test/` - Adding/modifying tests

**Examples:**
- `feature/user-authentication`
- `feature/recurring-trips`
- `fix/database-connection-timeout`
- `fix/refund-calculation`
- `hotfix/payment-gateway-error`
- `refactor/kubernetes-deployment-config`
- `refactor/extract-validation`
- `docs/api-endpoints`
- `test/booking-service-unit-tests`

**Best practices:**
- Use lowercase only
- Use hyphens `-` to separate words (not underscores)
- Keep descriptions short but meaningful
- Be specific enough to identify the work

## Commit Message Format

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat:` - New functionality
- `fix:` - Bug fix
- `refactor:` - Code refactoring (no behavior change)
- `docs:` - Documentation only
- `style:` - Formatting, no code logic change
- `test:` - Adding/modifying tests
- `chore:` - Dependencies, config, misc tasks
- `perf:` - Performance improvement

**Examples:**
```
feat(auth): add JWT token validation
fix(k8s): correct memory limits in deployment
refactor(api): simplify user controller logic
docs(readme): update installation instructions
chore(deps): upgrade firebase to v10.2
perf(booking): optimize segment query with index
style(auth): fix indentation in handler
test(payment): add refund calculation tests
```

**Best practices:**
- Use present imperative tense: "add" not "added"
- First line max 50-72 characters
- Describe WHAT, not HOW
- Scope is optional (module/component in parentheses)
- One commit = one logical change
- Commit often, push regularly

**Multi-line example:**
```
feat(booking): add multi-segment cancellation support

- Calculate refund per segment instead of total
- Update booking_segments status individually
- Add cancellation_reason to booking history

Closes #234
```

## Review Process

1. **Create PR** - Fill out the template completely
2. **Self-review** - Review your own code first, check the diff
3. **Request review** - Assign appropriate reviewer(s)
4. **Address feedback** - Respond to comments, push fixes
5. **Approval** - Get at least one approval before merging
6. **Merge** - Use "Squash and merge" to keep history clean

## Tips for Good PRs

**Keep PRs small and focused:**
- One feature or fix per PR
- Aim for under 400 lines changed
- Split large features into multiple PRs

**Make reviewers' lives easier:**
- Add comments on complex logic in your PR
- Highlight areas where you want specific feedback
- Respond to all comments, even if just "Done"

**Use draft PRs:**
- Create a draft PR early for visibility
- Mark as "Ready for review" when complete

## Questions?

If you're unsure about any of these guidelines, ask the team before submitting your PR. It's better to clarify upfront than to go through multiple revision cycles.