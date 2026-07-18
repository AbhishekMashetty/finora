"use client";

// Settings screen: currency + timezone against user-service via the
// gateway. GET/PUT /api/v1/users/me/settings. GetSettings always returns
// sane defaults (USD/UTC) rather than 404 for a user who's never saved
// settings, so this page never needs an empty-state branch — see
// user-service's GetSettings doc comment.

import { useEffect, useState, type FormEvent } from "react";
import { apiFetch, ApiError } from "@/lib/api";
import type { Settings } from "@/lib/types";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { SkeletonRows } from "@/components/ui/Skeleton";

// A short list of common IANA timezones, offered as soft <datalist>
// suggestions — not a strict dropdown, since the backend accepts any real
// IANA name (validated via Go's time.LoadLocation) and hardcoding an
// exhaustive list here would drift from that.
const TIMEZONE_SUGGESTIONS = [
  "UTC",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "Europe/London",
  "Europe/Paris",
  "Europe/Berlin",
  "Asia/Kolkata",
  "Asia/Tokyo",
  "Asia/Singapore",
  "Australia/Sydney",
];

export default function SettingsPage() {
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [currency, setCurrency] = useState("USD");
  const [timezone, setTimezone] = useState("UTC");
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const data = await apiFetch<{ settings: Settings }>("/api/v1/users/me/settings");
        if (!cancelled) {
          setCurrency(data.settings.currency);
          setTimezone(data.settings.timezone);
        }
      } catch (err) {
        if (!cancelled) {
          setLoadError(err instanceof ApiError ? err.message : "Could not load your settings.");
        }
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setSaveError(null);
    setSaved(false);
    setIsSaving(true);
    try {
      const data = await apiFetch<{ settings: Settings }>("/api/v1/users/me/settings", {
        method: "PUT",
        body: JSON.stringify({ currency, timezone }),
      });
      setCurrency(data.settings.currency);
      setTimezone(data.settings.timezone);
      setSaved(true);
    } catch (err) {
      setSaveError(err instanceof ApiError ? err.message : "Could not update your settings.");
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold text-ink-primary">Settings</h1>

      <Card className="mt-6 max-w-md">
        {isLoading && <SkeletonRows rows={2} />}
        {!isLoading && loadError && <p className="text-sm text-status-critical">{loadError}</p>}
        {!isLoading && !loadError && (
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Input
              id="settings-currency"
              label="Currency"
              required
              maxLength={3}
              value={currency}
              onChange={(e) => {
                setCurrency(e.target.value.toUpperCase());
                setSaved(false);
              }}
              placeholder="USD"
            />
            <div>
              <datalist id="timezone-suggestions">
                {TIMEZONE_SUGGESTIONS.map((tz) => (
                  <option key={tz} value={tz} />
                ))}
              </datalist>
              <Input
                id="settings-timezone"
                label="Timezone"
                required
                list="timezone-suggestions"
                value={timezone}
                onChange={(e) => {
                  setTimezone(e.target.value);
                  setSaved(false);
                }}
                placeholder="America/New_York"
              />
              <p className="mt-1 text-xs text-ink-muted">
                Any valid IANA timezone name (e.g. Europe/London).
              </p>
            </div>

            {saveError && <p className="text-sm text-status-critical">{saveError}</p>}
            {saved && <p className="text-sm text-status-good-text">Settings updated.</p>}

            <div>
              <Button type="submit" size="sm" disabled={isSaving}>
                {isSaving ? "Saving…" : "Save changes"}
              </Button>
            </div>
          </form>
        )}
      </Card>
    </div>
  );
}
