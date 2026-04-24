export type Page =
  | "dashboard"
  | "harness"
  | "problems"
  | "portfolios"
  | "decisions"
  | "jobs"
  | "tasks"
  | "settings";

const LEGACY_PAGE_REDIRECTS: Partial<Record<Page, Page>> = {
  decisions: "dashboard",
  problems: "dashboard",
};

const PAGE_TITLES: Record<Page, string> = {
  dashboard: "Core",
  decisions: "Core",
  harness: "Runtime",
  jobs: "Jobs",
  portfolios: "Comparison",
  problems: "Core",
  settings: "Settings",
  tasks: "Tasks",
};

export function normalizePage(page: Page): Page {
  const redirectedPage = LEGACY_PAGE_REDIRECTS[page];

  return redirectedPage ?? page;
}

export function resolveNavigation(page: Page, id?: string) {
  const normalizedPage = normalizePage(page);
  const selectedId = normalizedPage === page ? id ?? null : null;

  return { page: normalizedPage, selectedId };
}

export function getPageTitle(page: Page): string {
  const normalizedPage = normalizePage(page);
  const title = PAGE_TITLES[normalizedPage];

  return title;
}
