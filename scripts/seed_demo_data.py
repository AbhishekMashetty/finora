#!/usr/bin/env python3
"""Seed Finora with realistic-looking synthetic demo data.

Populates a demo user's accounts, categories, transactions, budgets, and
savings goals through the live gateway API — the exact same endpoints the
frontend uses, so there's nothing special to keep in sync. Every value is
synthetic, but shaped to look like a real user's data: real-style bank/card
account names, real-style merchant names per category, income paychecks on
a biweekly cadence, expense amounts scaled to what that kind of merchant
actually costs, and a couple of categories deliberately budgeted to be
"over" so the Reports page's budget-vs-actual bars and the notification
feed both have something real to show.

Zero third-party dependencies on purpose — this is a small, occasional dev
tool, not application code, and CLAUDE.md's "every dependency needs a
one-line justification" bar isn't worth spending on `requests` for a script
that only ever does simple JSON-over-HTTP calls; Python's stdlib
(urllib.request) is sufficient. Run with a plain `python3`, no `pip install`.

Usage:
    python3 scripts/seed_demo_data.py
    python3 scripts/seed_demo_data.py --email demo2@example.com --transactions 200
    python3 scripts/seed_demo_data.py --base-url http://localhost:8080 --seed 7

Re-running with the same --email adds more data on top of whatever that
user already has (registration failing with "already exists" falls back to
login, rather than aborting) — pass a different --email for a separate,
independent demo user instead.
"""

from __future__ import annotations

import argparse
import datetime
import json
import random
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from typing import Any


# --------------------------------------------------------------------------
# HTTP helper — a thin wrapper around urllib that speaks the shared/httpx
# envelope every Finora service responds with: {success, data, error,
# request_id}. Mirrors what frontend/lib/api.ts does for the browser.
# --------------------------------------------------------------------------

class ApiError(Exception):
    def __init__(self, status: int, code: str, message: str):
        super().__init__(f"{status} {code}: {message}")
        self.status = status
        self.code = code


def api_request(
    base_url: str,
    method: str,
    path: str,
    body: dict[str, Any] | None = None,
    token: str | None = None,
) -> dict[str, Any]:
    url = f"{base_url}{path}"
    data = json.dumps(body).encode("utf-8") if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", f"Bearer {token}")

    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            raw = resp.read()
            payload = json.loads(raw) if raw else {"success": True, "data": None}
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError:
            raise ApiError(e.code, "UNKNOWN", raw.decode("utf-8", "replace")) from e
        err = payload.get("error") or {}
        raise ApiError(e.code, err.get("code", "UNKNOWN"), err.get("message", "request failed")) from e

    if not payload.get("success", True):
        err = payload.get("error") or {}
        raise ApiError(200, err.get("code", "UNKNOWN"), err.get("message", "request failed"))
    return payload.get("data") or {}


# --------------------------------------------------------------------------
# Realistic demo data pools
# --------------------------------------------------------------------------

# POST /api/v1/accounts has no `balance` field (every account starts at 0 —
# expense-service never derives it from transactions either), so without an
# explicit follow-up PUT, a demo account with 80+ real-looking transactions
# would still show a $0 balance — which reads as broken, not realistic. Each
# account gets a plausible starting balance set right after creation
# instead. balance_range is (min, max); credit is negative (money owed),
# everything else positive.
ACCOUNTS = [
    {"name": "Chase Total Checking", "type": "checking", "currency": "USD", "balance_range": (2200, 8500)},
    {"name": "Ally Online Savings", "type": "savings", "currency": "USD", "balance_range": (4500, 26000)},
    {"name": "American Express Gold Card", "type": "credit", "currency": "USD", "balance_range": (-2100, -80)},
]

# Each category: (name, type, [merchant names], (min_amount, max_amount))
CATEGORIES: list[dict[str, Any]] = [
    {
        "name": "Groceries",
        "type": "expense",
        "merchants": ["Whole Foods Market", "Trader Joe's", "Safeway", "Costco Wholesale", "Kroger"],
        "amount": (18, 165),
        "monthly_count": (8, 12),
    },
    {
        "name": "Dining & Coffee",
        "type": "expense",
        "merchants": ["Starbucks", "Chipotle Mexican Grill", "Sweetgreen", "Blue Bottle Coffee", "Olive Garden", "Local Bistro"],
        "amount": (6, 68),
        "monthly_count": (10, 16),
    },
    {
        "name": "Transportation",
        "type": "expense",
        "merchants": ["Shell Gas Station", "Chevron", "Uber", "Lyft", "Bay Area Rapid Transit"],
        "amount": (9, 75),
        "monthly_count": (6, 10),
    },
    {
        "name": "Entertainment & Streaming",
        "type": "expense",
        "merchants": ["Netflix", "Spotify Premium", "AMC Theatres", "Steam", "Disney+"],
        "amount": (9, 45),
        "monthly_count": (3, 6),
    },
    {
        "name": "Utilities & Bills",
        "type": "expense",
        "merchants": ["PG&E", "Comcast Xfinity", "Verizon Wireless", "City Water & Sewer Dept"],
        "amount": (45, 210),
        "monthly_count": (3, 4),
    },
    {
        "name": "Rent & Housing",
        "type": "expense",
        "merchants": ["Greystar Property Management"],
        "amount": (1850, 2650),
        "monthly_count": (1, 1),
    },
    {
        "name": "Health & Fitness",
        "type": "expense",
        "merchants": ["Equinox Fitness", "CVS Pharmacy", "Kaiser Permanente Copay"],
        "amount": (15, 120),
        "monthly_count": (2, 4),
    },
    {
        "name": "Shopping",
        "type": "expense",
        "merchants": ["Amazon.com", "Target", "Best Buy", "Nike", "IKEA"],
        "amount": (14, 220),
        "monthly_count": (4, 8),
    },
    {
        "name": "Travel",
        "type": "expense",
        "merchants": ["Delta Air Lines", "Marriott Hotels", "Airbnb"],
        "amount": (110, 650),
        "monthly_count": (0, 1),
    },
    {
        "name": "Subscriptions",
        "type": "expense",
        "merchants": ["Amazon Prime", "iCloud Storage", "The New York Times", "Adobe Creative Cloud"],
        "amount": (5, 25),
        "monthly_count": (2, 4),
    },
]

INCOME_SOURCES = ["Acme Corp Payroll", "TechNova Inc Direct Deposit"]
INCOME_AMOUNT = (2400, 3600)  # per biweekly paycheck

# Budgets deliberately mix under, near, and over — so the Reports page's
# BudgetBar shows all three status colors, and "over" triggers a real
# notification. Category names match CATEGORIES above (case-insensitive
# match is how budget-service resolves them, per architecture/api-
# contracts.md, so exact case doesn't matter, but keep it human-readable).
BUDGETS = [
    {"category": "Groceries", "amount": 500, "period": "monthly"},        # likely under/near
    {"category": "Dining & Coffee", "amount": 250, "period": "monthly"},  # likely near/over
    {"category": "Transportation", "amount": 200, "period": "monthly"},
    {"category": "Entertainment & Streaming", "amount": 60, "period": "monthly"},  # likely over
    {"category": "Shopping", "amount": 300, "period": "monthly"},
]

GOALS = [
    {"name": "Emergency Fund", "target_amount": 10000, "current_amount": 3500, "months_out": 8},
    {"name": "Trip to Japan", "target_amount": 5000, "current_amount": 1200, "months_out": 5},
    {"name": "New MacBook Pro", "target_amount": 2500, "current_amount": 800, "months_out": 3},
]


def iso_date(d: datetime.date) -> str:
    return d.isoformat()


def months_from_now(n: int) -> datetime.date:
    today = datetime.date.today()
    month = today.month - 1 + n
    year = today.year + month // 12
    month = month % 12 + 1
    day = min(today.day, 28)
    return datetime.date(year, month, day)


@dataclass
class SeedResult:
    accounts: list[dict] = field(default_factory=list)
    categories: list[dict] = field(default_factory=list)
    transactions: int = 0
    budgets: list[dict] = field(default_factory=list)
    goals: list[dict] = field(default_factory=list)
    notifications: int = 0


def register_or_login(base_url: str, email: str, password: str, name: str) -> str:
    try:
        api_request(base_url, "POST", "/api/v1/auth/register", {"email": email, "password": password, "name": name})
        print(f"  registered new user {email}")
    except ApiError as e:
        if e.code == "CONFLICT":
            print(f"  {email} already exists — logging in and adding more data on top")
        else:
            raise

    data = api_request(base_url, "POST", "/api/v1/auth/login", {"email": email, "password": password})
    return data["access_token"]


def seed(base_url: str, token: str, months: int, transaction_count: int, rng: random.Random) -> SeedResult:
    result = SeedResult()

    print("Creating accounts...")
    account_ids: list[str] = []
    for acc in ACCOUNTS:
        data = api_request(
            base_url, "POST", "/api/v1/accounts",
            {"name": acc["name"], "type": acc["type"], "currency": acc["currency"]},
            token=token,
        )
        account_id = data["account"]["id"]
        account_ids.append(account_id)

        balance = round(rng.uniform(*acc["balance_range"]), 2)
        api_request(
            base_url, "PUT", f"/api/v1/accounts/{account_id}",
            {"name": acc["name"], "type": acc["type"], "currency": acc["currency"], "balance": balance},
            token=token,
        )

        result.accounts.append(acc)
        print(f"  + {acc['name']} ({acc['type']}, balance {balance:.2f})")
    checking_id = account_ids[0]
    credit_id = account_ids[2]

    print("Creating categories...")
    category_ids: dict[str, str] = {}
    for cat in CATEGORIES:
        data = api_request(
            base_url, "POST", "/api/v1/categories",
            {"name": cat["name"], "type": cat["type"]}, token=token,
        )
        category_ids[cat["name"]] = data["category"]["id"]
        result.categories.append(cat)
        print(f"  + {cat['name']} ({cat['type']})")
    # A couple of income categories too, for salary transactions.
    salary_category_id = api_request(
        base_url, "POST", "/api/v1/categories", {"name": "Salary", "type": "income"}, token=token,
    )["category"]["id"]

    print(f"Creating ~{transaction_count} transactions over the last {months} months...")
    today = datetime.date.today()
    window_start = today - datetime.timedelta(days=months * 30)

    # Income: a paycheck roughly every two weeks, into checking.
    paycheck_date = window_start
    tx_created = 0
    while paycheck_date <= today:
        amount = round(rng.uniform(*INCOME_AMOUNT), 2)
        api_request(
            base_url, "POST", "/api/v1/transactions",
            {
                "account_id": checking_id,
                "category_id": salary_category_id,
                "type": "income",
                "amount": amount,
                "currency": "USD",
                "date": iso_date(paycheck_date),
                "note": rng.choice(INCOME_SOURCES),
            },
            token=token,
        )
        tx_created += 1
        paycheck_date += datetime.timedelta(days=14)

    # Expenses: distributed per category per month, real merchant notes,
    # amounts scaled per category, split across checking/credit realistically
    # (rent/utilities from checking, day-to-day spend on the credit card).
    checking_categories = {"Rent & Housing", "Utilities & Bills"}

    for cat in CATEGORIES:
        for month_offset in range(months):
            month_date = today - datetime.timedelta(days=30 * month_offset)
            count = rng.randint(*cat["monthly_count"])
            for _ in range(count):
                if tx_created >= transaction_count and month_offset > 0:
                    break
                day_offset = rng.randint(0, 27)
                tx_date = month_date - datetime.timedelta(days=day_offset)
                if tx_date < window_start or tx_date > today:
                    continue
                amount = round(rng.uniform(*cat["amount"]), 2)
                account_id = checking_id if cat["name"] in checking_categories else rng.choice([checking_id, credit_id])
                api_request(
                    base_url, "POST", "/api/v1/transactions",
                    {
                        "account_id": account_id,
                        "category_id": category_ids[cat["name"]],
                        "type": "expense",
                        "amount": amount,
                        "currency": "USD",
                        "date": iso_date(tx_date),
                        "note": rng.choice(cat["merchants"]),
                    },
                    token=token,
                )
                tx_created += 1
        if tx_created % 20 < 3:
            print(f"  ...{tx_created} transactions so far")

    result.transactions = tx_created
    print(f"  {tx_created} transactions created")

    print("Creating budgets...")
    for b in BUDGETS:
        api_request(base_url, "POST", "/api/v1/budgets", b, token=token)
        result.budgets.append(b)
        print(f"  + {b['category']}: {b['amount']}/{b['period']}")

    print("Creating savings goals...")
    for g in GOALS:
        target_date = iso_date(months_from_now(g["months_out"]))
        created = api_request(
            base_url, "POST", "/api/v1/goals",
            {"name": g["name"], "target_amount": g["target_amount"], "target_date": target_date},
            token=token,
        )
        goal_id = created["goal"]["id"]
        api_request(
            base_url, "PUT", f"/api/v1/goals/{goal_id}",
            {
                "name": g["name"],
                "target_amount": g["target_amount"],
                "target_date": target_date,
                "current_amount": g["current_amount"],
            },
            token=token,
        )
        result.goals.append(g)
        print(f"  + {g['name']}: {g['current_amount']}/{g['target_amount']} by {target_date}")

    # Force the current month's overspend-notification trigger (see
    # architecture/api-contracts.md's budget-service -> notification-service
    # subsection) so the Notifications page has something real to show too.
    print("Triggering this month's report (populates budget-vs-actual + any overspend notifications)...")
    month_from = today.replace(day=1)
    next_month = (month_from.replace(day=28) + datetime.timedelta(days=4)).replace(day=1)
    month_to = next_month - datetime.timedelta(days=1)
    api_request(
        base_url, "GET",
        f"/api/v1/reports/summary?from={iso_date(month_from)}&to={iso_date(month_to)}",
        token=token,
    )
    notifications = api_request(base_url, "GET", "/api/v1/notifications", token=token)
    result.notifications = len(notifications.get("notifications", []))

    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--base-url", default="http://localhost:8080", help="Gateway base URL (default: %(default)s)")
    parser.add_argument("--email", default="demo@example.com", help="Demo user email (default: %(default)s)")
    parser.add_argument("--password", default="DemoPass123!", help="Demo user password (default: %(default)s)")
    parser.add_argument("--name", default="Demo User", help="Demo user display name (default: %(default)s)")
    parser.add_argument("--months", type=int, default=3, help="Months of transaction history to generate (default: %(default)s)")
    parser.add_argument("--transactions", type=int, default=140, help="Approximate total transaction count (default: %(default)s)")
    parser.add_argument("--seed", type=int, default=None, help="Random seed, for reproducible runs (default: random each time)")
    args = parser.parse_args()

    rng = random.Random(args.seed)

    print(f"Seeding Finora demo data at {args.base_url}")
    print(f"  user: {args.email}")
    token = register_or_login(args.base_url, args.email, args.password, args.name)

    result = seed(args.base_url, token, args.months, args.transactions, rng)

    print()
    print("Done.")
    print(f"  {len(result.accounts)} accounts, {len(result.categories) + 1} categories, "
          f"{result.transactions} transactions, {len(result.budgets)} budgets, "
          f"{len(result.goals)} goals, {result.notifications} notification(s)")
    print()
    print(f"Log in at http://localhost:3000/login with:")
    print(f"  email:    {args.email}")
    print(f"  password: {args.password}")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except ApiError as e:
        print(f"\nAPI error: {e}", file=sys.stderr)
        sys.exit(1)
    except urllib.error.URLError as e:
        print(f"\nCould not reach the gateway — is `docker compose up` running? ({e})", file=sys.stderr)
        sys.exit(1)
