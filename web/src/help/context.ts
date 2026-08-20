// Maps app routes to the help articles most relevant on that page —
// surfaced as "Suggested for this page" when the help panel opens.
// Order matters: first matching pattern wins (note /org-chart before /org).

const ROUTE_HELP: Array<{ pattern: RegExp; articles: string[] }> = [
  { pattern: /^\/dashboard/, articles: ["welcome-tour", "approving-time-off", "onboarding-checklists"] },
  { pattern: /^\/directory/, articles: ["directory-basics", "worker-profiles"] },
  { pattern: /^\/people\//, articles: ["worker-profiles", "assignment-changes", "compensation-records"] },
  { pattern: /^\/org-chart/, articles: ["org-chart", "assignment-changes"] },
  { pattern: /^\/org/, articles: ["org-structure", "org-chart"] },
  { pattern: /^\/time-off/, articles: ["requesting-time-off", "approving-time-off", "leave-types-accruals"] },
  { pattern: /^\/onboarding/, articles: ["onboarding-checklists", "checklist-templates"] },
  { pattern: /^\/analytics/, articles: ["analytics-overview", "compensation-records"] },
  { pattern: /^\/team/, articles: ["team-access", "roles-permissions"] },
];

export function helpArticlesFor(pathname: string): string[] {
  return ROUTE_HELP.find((r) => r.pattern.test(pathname))?.articles ?? ["welcome-tour"];
}

// Every article id referenced by the map (useful for integrity checks).
export function allMappedArticleIds(): string[] {
  return [...new Set(ROUTE_HELP.flatMap((r) => r.articles))];
}
