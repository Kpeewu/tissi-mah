#!/bin/bash
set -e

echo "🧪 Running notification-service tests..."
go test ./tests/unit/... -v -race -coverprofile=coverage.out
echo "✅ All tests passed!"
