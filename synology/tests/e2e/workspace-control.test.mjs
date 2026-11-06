import assert from 'node:assert/strict';
import test from 'node:test';
import { chromium } from 'playwright-core';
import { fixtureWorkspaceControl } from './workspace-control.mjs';

async function withPage(action) {
  const browser = await chromium.launch({ args: ['--no-sandbox'],
    ...(process.env.OVIS_E2E_BROWSER ? { executablePath: process.env.OVIS_E2E_BROWSER } : {}) });
  try { await action(await browser.newPage()); } finally { await browser.close(); }
}

// The header and sidebar deliberately share the exact label, as the real app does.
test('selects the workspace control so onboarding steps aside, leaving the header alone', () => withPage(async page => {
  await page.setContent(`<header><span>fixture</span></header>
    <aside class="sidebar"><div class="workspace-select" role="button"><span>fixture</span></div></aside>
    <section id="guide">Setup guide</section><div id="terminal" hidden>Terminal</div>`);
  await page.evaluate(() => {
    document.querySelector('header').onclick = () => { document.body.dataset.headerClicked = 'yes'; };
    document.querySelector('.workspace-select').onclick = () => {
      document.querySelector('#guide').hidden = true;
      document.querySelector('#terminal').hidden = false;
    };
  });
  await fixtureWorkspaceControl(page, 'fixture').click();
  assert.equal(await page.locator('#guide').isVisible(), false);
  assert.equal(await page.locator('#terminal').isVisible(), true);
  assert.equal(await page.locator('body').getAttribute('data-header-clicked'), null);
}));

test('requires an exact sidebar label and refuses an ambiguous fixture', () => withPage(async page => {
  await page.setContent(`<header><span>fixture</span></header><aside class="sidebar">
    <div class="workspace-select" role="button"><span>fixture-other</span></div>
    <div class="workspace-select" role="button"><span>fixture</span></div></aside>
    <div class="workspace-select" role="button"><span>fixture</span></div>`);
  assert.equal(await fixtureWorkspaceControl(page, 'fixture').count(), 1);
  await page.locator('.sidebar').evaluate(node => node.appendChild(node.lastElementChild.cloneNode(true)));
  assert.equal(await fixtureWorkspaceControl(page, 'fixture').count(), 2);
  await assert.rejects(fixtureWorkspaceControl(page, 'fixture').click({ timeout: 1000 }), /strict mode violation/);
}));
