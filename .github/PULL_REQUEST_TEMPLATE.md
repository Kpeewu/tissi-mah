## Description

<!-- Provide a clear and concise description of your changes -->

## Type of Change

<!-- Check the relevant option -->

- [ ] ✨ Feature (`feat`) - New functionality
- [ ] 🐛 Fix (`fix`) - Bug correction
- [ ] 🚑 Hotfix (`hotfix`) - Urgent production fix
- [ ] ♻️ Refactor (`refactor`) - Code improvement without behavior change
- [ ] 📚 Docs (`docs`) - Documentation update
- [ ] 🎨 Style (`style`) - Formatting, no code change
- [ ] ✅ Test (`test`) - Adding or modifying tests
- [ ] 🔧 Chore (`chore`) - Dependencies, config, CI/CD, Helm charts
- [ ] ⚡ Perf (`perf`) - Performance improvement
- [ ] 🏗️ Infrastructure - Terraform, Kubernetes configuration

## Impacted Service(s)

<!-- Check all services affected by this PR -->

- [ ] auth-service
- [ ] user-service
- [ ] trips-service
- [ ] booking-service
- [ ] payment-service
- [ ] rating-service
- [ ] vehicle-service
- [ ] file-service
- [ ] notification-service
- [ ] stats-service
- [ ] geolocation-service
- [ ] infrastructure (Terraform/Helm)
- [ ] shared/common code

## Related Feature / Module

<!-- Describe which feature or module is impacted (e.g., "User registration", "Trip booking flow", "Payment refunds") -->

## How to Test

<!-- Provide step-by-step instructions to test your changes -->

1. 
2. 
3. 

### Expected Result

<!-- What should happen when testing? -->

## Checklist

<!-- Ensure all applicable items are checked before requesting review -->

### Code Quality
- [ ] My code follows the project's coding standards
- [ ] I have run the linter (`make lint`)
- [ ] I have formatted my code (`make fmt`)
- [ ] No secrets or credentials are hardcoded

### Testing
- [ ] I have tested my changes locally
- [ ] All existing tests pass (`make test`)
- [ ] I have added tests for new functionality (if applicable)

### Documentation
- [ ] I have updated the documentation (if applicable)
- [ ] New environment variables are documented

### Database & API
- [ ] Database migrations are included (if schema changed)
- [ ] Proto files are updated and regenerated (if gRPC contracts changed)

### Deployment
- [ ] Helm values are updated for all environments (if new config added)
- [ ] I have verified the CI pipeline passes

## Screenshots / Logs

<!-- If applicable, add screenshots or relevant logs -->

## Additional Notes

<!-- Any additional context, concerns, or notes for reviewers -->

---

**Reviewer Tips:**
- Focus on: 
- Pay attention to: