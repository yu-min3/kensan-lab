#!/usr/bin/env python3
"""Regenerate the screenshots in docs/getting-started/assets/.

Screenshots rot: a component gets restyled, a dashboard gains a row, and the
picture in the guide quietly starts describing a cluster nobody runs any more.
This script exists so that fixing that is one command against a live explore
cluster rather than an afternoon of cropping.

    make try                                      # stand a cluster up
    DEMO_PASSWORD=... scripts/explore-screenshots.py argocd-tree demo-application

Everything it photographs is reached the way the guide tells a visitor to reach
it — through the gateway, signing in as `demo` at Keycloak — so a screenshot
that cannot be taken is a sign the documented path is broken.

Two of them contradict each other about when to run, so every run names what it
captures:

    DEMO_PASSWORD=... scripts/explore-screenshots.py argocd-tree   # before
    ... run the Golden Path and leave its pull request open ...
    DEMO_PASSWORD=... scripts/explore-screenshots.py gitea-platform-pr

`argocd-tree` illustrates step 3, where the visitor has not created anything
yet, so a generated application must not be in the picture. `gitea-platform-pr`
illustrates step 6 and needs the pull request that only exists after the Golden
Path has run. A run with no names is refused rather than silently producing a
stale image.

Needs playwright with a real Chrome (`channel="chrome"`); no bundled browser
download is required.
"""

from __future__ import annotations

import base64
import os
import pathlib
import sys

from playwright.sync_api import (
    Page,
    TimeoutError as PlaywrightTimeout,
    sync_playwright,
)

OUT = pathlib.Path(__file__).resolve().parent.parent / "docs" / "getting-started" / "assets"
DEMO_USER = "demo"
DEMO_PASSWORD = os.environ.get("DEMO_PASSWORD", "")

# Wide enough that Argo CD's tree and Grafana's panels are not squeezed into a
# mobile layout, and short enough that the result is readable inline in a
# Markdown page rather than a wall the reader has to scroll past.
VIEWPORT = {"width": 1440, "height": 900}


# Argo CD and Grafana authenticate themselves rather than sitting behind the
# gateway's gate, so they show their own login page with a button to Keycloak
# instead of redirecting straight there. A visitor clicks it; so does this.
SSO_BUTTONS = ["LOG IN VIA KEYCLOAK", "Sign in with Keycloak"]


def sign_in(page: Page) -> None:
    """Get past whatever login is showing. A no-op once signed in."""
    # Argo CD and Backstage render their login through a JavaScript bundle, so
    # the button does not exist at domcontentloaded. Looking too early finds
    # nothing, and finding nothing is indistinguishable from being signed in
    # already — which is how this script first produced seven screenshots of
    # login pages.
    page.wait_for_timeout(2500)

    for label in SSO_BUTTONS:
        button = page.get_by_text(label, exact=False).first
        try:
            if button.is_visible(timeout=2000):
                button.click()
                # Not networkidle: Argo CD and Grafana poll continuously, so
                # the network never goes quiet and the wait would time out on a
                # page that is perfectly ready.
                page.wait_for_timeout(2500)
                break
        except PlaywrightTimeout:
            continue

    try:
        page.wait_for_selector("#username", timeout=8000)
    except PlaywrightTimeout:
        return
    page.fill("#username", DEMO_USER)
    page.fill("#password", DEMO_PASSWORD)
    page.click("#kc-login")
    page.wait_for_timeout(4000)


def shot(page: Page, url: str, name: str, *, wait_for: str | None = None,
         settle: int = 2500) -> None:
    page.goto(url, wait_until="domcontentloaded")
    sign_in(page)
    # An OIDC round trip does not always come back to where it started: Argo CD
    # returns to its application list rather than the deep link that triggered
    # the login. Asking for the page a second time costs nothing once the
    # session exists.
    if not page.url.startswith(url):
        page.goto(url, wait_until="domcontentloaded")
    if wait_for:
        page.wait_for_selector(wait_for, timeout=30000)
    page.wait_for_timeout(settle)

    # A screenshot of a login page is a failure that looks like a success in the
    # file listing. Refuse to write one.
    body = page.inner_text("body")
    for marker in ("LOG IN VIA KEYCLOAK", "Sign in to your account", "Sign in with Keycloak"):
        if marker in body:
            raise SystemExit(
                f"{name}: still on a login page — sign_in did not take effect.\n"
                f"  url: {page.url}"
            )

    path = OUT / f"{name}.png"
    page.screenshot(path=str(path))
    print(f"  {path.relative_to(OUT.parent.parent.parent)}")


def sign_in_to_gitea(page: Page) -> str:
    """Sign in to Gitea's separate local session with the demo credentials."""
    credentials = base64.b64encode(
        f"{DEMO_USER}:{DEMO_PASSWORD}".encode()
    ).decode()
    page.goto("https://gitea.127-0-0-1.sslip.io/user/login", wait_until="domcontentloaded")
    username = page.locator('input[name="user_name"]')
    if username.count():
        username.fill(DEMO_USER)
        page.locator('input[name="password"]').fill(DEMO_PASSWORD)
        # Gitea's button intentionally has no `type=submit`; submit the form
        # itself so a cosmetic button markup change cannot break screenshot
        # regeneration.
        page.locator('form[action="/user/login"]').evaluate("form => form.submit()")
        page.wait_for_url(lambda url: "/user/login" not in url, timeout=20000)

    # Gitea stores a language on the account and prefers it over the context's
    # Accept-Language, so a maintainer whose browser once visited this server in
    # another language keeps regenerating the screenshot in that language. Set
    # it every run rather than only on the run that signs in.
    status = page.evaluate(
        """credentials => fetch('/api/v1/user/settings', {
          method: 'PATCH',
          headers: {
            'Authorization': `Basic ${credentials}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({language: 'en-US'}),
        }).then(response => response.status)""",
        credentials,
    )
    if status != 200:
        raise SystemExit(f"Gitea rejected the English locale update (HTTP {status})")
    pull_url = page.evaluate(
        """credentials => fetch('/api/v1/repos/demo/kensan-lab/pulls?state=open', {
          headers: {'Authorization': `Basic ${credentials}`},
        }).then(response => response.json()).then(pulls => {
          const pull = pulls.find(item => item.title.startsWith('Add Application:'));
          return pull ? pull.html_url : null;
        })""",
        credentials,
    )
    if not pull_url:
        raise SystemExit(
            "gitea-platform-pr: no Golden Path pull request is open.\n"
            "  Run the Golden Path first and leave its pull request unmerged."
        )
    return pull_url


# In capture order. `main` refuses a name that is not here rather than taking
# every screenshot when one is misspelled.
NAMES = (
    "demo-application",
    "argocd-tree",
    "gitea-platform-pr",
)


def main(argv: list[str]) -> int:
    if not DEMO_PASSWORD:
        print("DEMO_PASSWORD is not set — pass the one `make try` printed.", file=sys.stderr)
        return 2
    if not argv:
        print(
            "name the screenshot phase explicitly:\n"
            "  before Golden Path: argocd-tree demo-application\n"
            "  with an open PR:    gitea-platform-pr",
            file=sys.stderr,
        )
        return 2
    unknown = [name for name in argv if name not in NAMES]
    if unknown:
        print(f"not a screenshot: {', '.join(unknown)}\n"
              f"  known: {', '.join(NAMES)}", file=sys.stderr)
        return 2
    wanted = set(argv)
    OUT.mkdir(parents=True, exist_ok=True)

    with sync_playwright() as p:
        browser = p.chromium.launch(channel="chrome")
        # The certificate is signed by a CA the cluster invented, which is the
        # documented behaviour rather than a problem to fix here.
        context = browser.new_context(
            viewport=VIEWPORT,
            ignore_https_errors=True,
            locale="en-US",
        )
        page = context.new_page()

        print("taking screenshots:")

        if wanted & {"demo-application", "argocd-tree"}:
            page.goto("https://demo.127-0-0-1.sslip.io/", wait_until="domcontentloaded")
            sign_in(page)

        if "demo-application" in wanted:
            shot(page, "https://demo.127-0-0-1.sslip.io/",
                 "demo-application", settle=4000)

        # No selector waits below: these UIs render their content into
        # containers whose text a locator does not always see, and a screenshot
        # only needs the page to have settled. A blank result is obvious in
        # review, which a false-negative selector wait is not.
        if "argocd-tree" in wanted:
            shot(page, "https://argocd.127-0-0-1.sslip.io/applications/argocd/explore-root",
                 "argocd-tree", settle=7000)

        # Gitea intentionally has a separate local session. Capture the pull
        # request itself rather than the list, so the screenshot shows what the
        # platform administrator is asked to review.
        if "gitea-platform-pr" in wanted:
            pull_url = sign_in_to_gitea(page)
            shot(page, pull_url, "gitea-platform-pr", settle=4000)

        context.close()
        browser.close()
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
