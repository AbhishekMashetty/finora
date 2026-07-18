# scripts/

Helper scripts for local development. Not part of any service's build —
nothing here ships in a Docker image.

## `seed_demo_data.py`

Populates a demo user's accounts, categories, transactions, budgets, and
savings goals through the live gateway API, so the frontend has real-
looking data to click through instead of empty screens. Every value is
synthetic, but shaped to look real: actual-style bank/card account names,
merchant names per category, income paychecks on a biweekly cadence, and a
couple of categories deliberately budgeted to go "over" so the Reports
page's budget-vs-actual bars and the notification feed both have something
to show.

Requires only Python 3's standard library — no `pip install` needed.

```bash
# bring the stack up first
docker compose up --build -d

# then, from the repo root:
python3 scripts/seed_demo_data.py
```

Logs in at `http://localhost:3000/login` afterward with `demo@example.com`
/ `DemoPass123!` (both overridable via `--email`/`--password`). Run
`python3 scripts/seed_demo_data.py --help` for all options (transaction
count, months of history, a random seed for reproducible runs, a different
`--base-url` for a non-default gateway port).

Re-running with the same `--email` adds more data on top of whatever that
user already has — pass a different `--email` for a separate, independent
demo user instead.
