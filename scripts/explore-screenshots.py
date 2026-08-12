#!/usr/bin/env python3
"""Regenerate the screenshots in docs/getting-started/assets/.

Screenshots rot: a component gets restyled, a dashboard gains a row, and the
picture in the guide quietly starts describing a cluster nobody runs any more.
This script exists so that fixing that is one command against a live explore
cluster rather than an afternoon of cropping.

    scripts/explore-up.sh --rev <branch>          # stand a cluster up
    DEMO_PASSWORD=... scripts/explore-screenshots.py

Everything it photographs is reached the way the guide tells a visitor to reach
it — through the gateway, signing in as `demo` at Keycloak — so a screenshot
that cannot be taken is a sign the documented path is broken.

Needs playwright with a real Chrome (`channel="chrome"`); no bundled browser
download is required.
"""

from __future__ import annotations

import os
import pathlib
import sys

from playwright.sync_api import (
    Error as PlaywrightError,
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


def main() -> int:
    if not DEMO_PASSWORD:
        print("DEMO_PASSWORD is not set — pass the one `make try` printed.", file=sys.stderr)
        return 2
    OUT.mkdir(parents=True, exist_ok=True)

    with sync_playwright() as p:
        browser = p.chromium.launch(channel="chrome")
        # The certificate is signed by a CA the cluster invented, which is the
        # documented behaviour rather than a problem to fix here.
        context = browser.new_context(viewport=VIEWPORT, ignore_https_errors=True)
        page = context.new_page()

        print("taking screenshots:")

        # Unauthenticated first: this is the page a visitor meets before they
        # have signed in anywhere, and taking it after login would be impossible.
        page.goto("https://demo.127-0-0-1.sslip.io/", wait_until="domcontentloaded")
        page.wait_for_selector("#username", timeout=30000)
        page.wait_for_timeout(1500)
        (OUT / "keycloak-login.png").write_bytes(page.screenshot())
        print(f"  docs/getting-started/assets/keycloak-login.png")

        sign_in(page)

        # No selector waits below: these UIs render their content into
        # containers whose text a locator does not always see, and a screenshot
        # only needs the page to have settled. A blank result is obvious in
        # review, which a false-negative selector wait is not.
        shot(page, "https://argocd.127-0-0-1.sslip.io/applications/argocd/explore-root",
             "argocd-tree", settle=7000)

        # Backstage runs its own sign-in rather than sitting behind the gate, so
        # it needs its own click: the button opens a popup, the popup carries the
        # Keycloak form, and it closes itself once the identity is issued.
        page.goto("https://backstage.127-0-0-1.sslip.io/", wait_until="domcontentloaded")
        page.wait_for_timeout(3000)
        button = page.locator("button", has_text="SIGN IN").first
        if button.count():
            with page.expect_popup(timeout=20000) as popup:
                button.click()
            window = popup.value
            window.wait_for_selector("#username", timeout=20000)
            window.fill("#username", DEMO_USER)
            window.fill("#password", DEMO_PASSWORD)
            try:
                window.click("#kc-login")
            except PlaywrightError:
                # The popup closes the moment the identity is issued, which
                # races the click's own acknowledgement. The result shows up on
                # the main page either way.
                pass
            page.wait_for_timeout(9000)
        shot(page, "https://backstage.127-0-0-1.sslip.io/create",
             "backstage-create", settle=6000)
        # Backstage is missing on purpose. The image explore pins
        # (ghcr.io/yu-min3/kensan-lab/backstage) and the one bare metal pins
        # (ghcr.io/yu-min3/backstage) are different builds sharing the tag
        # v0.0.12, and only the second has the single sign-on frontend. Until
        # that is reconciled a screenshot here would show a guest card behind an
        # SSO gate, which describes neither cluster.
        # The dashboard itself rather than the list of them. `kiosk` drops Grafana's chrome so
        # the picture is the panels rather than a navigation bar, and the fixed
        # window keeps two runs comparable.
        shot(page,
             "https://grafana.127-0-0-1.sslip.io/d/cluster-health/cluster-health"
             "?orgId=1&from=now-30m&to=now&kiosk",
             "grafana-cluster-health", settle=9000)


        context.close()
        browser.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
