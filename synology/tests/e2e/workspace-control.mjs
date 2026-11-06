// The header repeats the label, but only the sidebar control selects a pane and
// tells an open Setup Guide to step aside. Do not choose an arbitrary text match.
export function fixtureWorkspaceControl(page, label) {
  return page.locator('.sidebar .workspace-select')
    .filter({ has: page.getByText(label, { exact: true }) });
}
