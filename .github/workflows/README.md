# GitHub Actions Workflows

This directory contains GitHub Actions workflows for the go-ethereum repository.

## Workflows

### sync-upstream.yml

**Purpose**: Automatically sync this repository with upstream releases from
[ethereum/go-ethereum](https://github.com/ethereum/go-ethereum).

**Triggers**:
- **Scheduled**: Runs daily at 00:00 UTC to check for new releases
- **Manual**: Can be triggered manually via workflow_dispatch

**How it works**:

1. **Checkout**: Fetches the repository with full history
2. **Add Upstream Remote**: Adds ethereum/go-ethereum as upstream remote
3. **Detect New Releases**: Checks if there are new release tags from upstream
4. **Merge**: If a new release is detected:
   - Attempts to automatically merge the upstream release
   - If successful, pushes the merged changes to the repository
   - If merge conflicts occur, creates an issue with manual merge instructions
5. **Summary**: Generates a summary of the sync operation

**Manual Trigger**:

To manually trigger the workflow:
1. Go to the Actions tab in GitHub
2. Select "Sync with Upstream Releases"
3. Click "Run workflow"

**Handling Merge Conflicts**:

If automatic merging fails due to conflicts, the workflow will:
1. Create an issue labeled with `upstream-sync` and `merge-conflict`
2. Provide instructions for manual resolution
3. Include a link to the upstream release

To manually resolve conflicts:
```bash
git remote add upstream https://github.com/ethereum/go-ethereum.git
git fetch upstream --tags
git merge <tag-name>
# Resolve conflicts in your editor
git add .
git commit
git push
```

**Notes**:
- The workflow only syncs release tags, not all commits
- Merge conflicts must be resolved manually
- The workflow requires `GITHUB_TOKEN` with write permissions
