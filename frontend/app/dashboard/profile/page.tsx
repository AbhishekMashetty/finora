"use client";

// Profile screen: view + edit the caller's own profile against
// user-service via the gateway. GET/PUT /api/v1/users/me. Email is
// immutable (no endpoint exists to change it — changing the account's login
// identity is a bigger, separate concern than editing a display name, not
// implemented anywhere in the fleet), rendered read-only.

import { useEffect, useState, type FormEvent } from "react";
import { apiFetch, ApiError } from "@/lib/api";
import type { User } from "@/lib/types";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { UserIcon } from "@/components/icons";

export default function ProfilePage() {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [name, setName] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const data = await apiFetch<{ user: User }>("/api/v1/users/me");
        if (!cancelled) {
          setUser(data.user);
          setName(data.user.name);
        }
      } catch (err) {
        if (!cancelled) {
          setLoadError(err instanceof ApiError ? err.message : "Could not load your profile.");
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
      const data = await apiFetch<{ user: User }>("/api/v1/users/me", {
        method: "PUT",
        body: JSON.stringify({ name }),
      });
      setUser(data.user);
      setSaved(true);
    } catch (err) {
      setSaveError(err instanceof ApiError ? err.message : "Could not update your profile.");
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold text-ink-primary">Profile</h1>

      <Card className="mt-6 max-w-md">
        {isLoading && <SkeletonRows rows={3} />}
        {!isLoading && loadError && <p className="text-sm text-status-critical">{loadError}</p>}
        {!isLoading && !loadError && user && (
          <div className="flex flex-col gap-4">
            <div className="flex items-center gap-3">
              <span className="flex h-10 w-10 items-center justify-center rounded-full bg-brand/10 text-brand">
                <UserIcon size={20} />
              </span>
              <div>
                <p className="text-sm font-medium text-ink-primary">{user.name}</p>
                <p className="text-xs text-ink-muted">{user.email}</p>
              </div>
            </div>

            <form onSubmit={handleSubmit} className="flex flex-col gap-3">
              <Input
                id="profile-email"
                label="Email"
                value={user.email}
                disabled
                title="Email cannot be changed here"
              />
              <Input
                id="profile-name"
                label="Name"
                required
                maxLength={100}
                value={name}
                onChange={(e) => {
                  setName(e.target.value);
                  setSaved(false);
                }}
              />

              {saveError && <p className="text-sm text-status-critical">{saveError}</p>}
              {saved && <p className="text-sm text-status-good-text">Profile updated.</p>}

              <div>
                <Button type="submit" size="sm" disabled={isSaving || name.trim() === ""}>
                  {isSaving ? "Saving…" : "Save changes"}
                </Button>
              </div>
            </form>
          </div>
        )}
      </Card>
    </div>
  );
}
