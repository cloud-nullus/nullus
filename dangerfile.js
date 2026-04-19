const changedFiles = [...danger.git.created_files, ...danger.git.modified_files];
const pr = danger.github.pr;

const body = (pr.body || "").trim();
if (body.length < 20) {
  warn("PR description is too short. Please add context, impact, and test notes.");
}

if (changedFiles.length > 40) {
  warn(`Large PR detected: ${changedFiles.length} changed files. Consider splitting for easier review.`);
}

if ((pr.additions || 0) + (pr.deletions || 0) > 1200) {
  warn(
    `Large diff detected: +${pr.additions || 0} / -${pr.deletions || 0}. Consider splitting the PR for safer review.`,
  );
}

const sourceTouched = changedFiles.some(
  (file) => file.startsWith("internal/") || file.startsWith("cmd/") || file.startsWith("web/src/"),
);

const testTouched = changedFiles.some((file) => {
  return (
    file.endsWith("_test.go") ||
    file.endsWith(".test.ts") ||
    file.endsWith(".test.tsx") ||
    file.endsWith(".spec.ts") ||
    file.endsWith(".spec.tsx") ||
    file.startsWith("e2e/")
  );
});

if (sourceTouched && !testTouched) {
  warn("Source code changed but no test files were updated. Please confirm test impact in the PR description.");
}

if (changedFiles.some((file) => file.startsWith(".github/workflows/"))) {
  message("Workflow changes detected. Please double-check token permissions and fork PR safety.");
}
