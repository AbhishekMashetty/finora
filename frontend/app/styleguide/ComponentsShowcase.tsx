"use client";

import { Trash2 } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Input } from "@/components/ui/Input";
import { Tooltip } from "@/components/ui/Tooltip";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/Dialog";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/DropdownMenu";

export function ComponentsShowcase() {
  return (
    <div className="flex flex-col gap-6 rounded-card border border-hairline bg-surface p-6">
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="primary">Primary</Button>
        <Button variant="secondary">Secondary</Button>
        <Button variant="danger">Danger</Button>
        <Button variant="ghost">Ghost</Button>
        <Button variant="primary" size="sm">
          Small
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Badge status="good">Good</Badge>
        <Badge status="warning">Warning</Badge>
        <Badge status="serious">Serious</Badge>
        <Badge status="critical">Critical</Badge>
        <Badge status="neutral">Neutral</Badge>
      </div>

      <div className="max-w-xs">
        <Input label="Email" placeholder="you@example.com" />
      </div>
      <div className="max-w-xs">
        <Input label="Amount" defaultValue="not a number" error="Enter a valid amount" />
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Tooltip content="Hover or focus me">
          <Button variant="secondary" size="sm">
            Hover for tooltip
          </Button>
        </Tooltip>

        <Dialog>
          <DialogTrigger asChild>
            <Button variant="secondary" size="sm">
              Open dialog
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Example dialog</DialogTitle>
              <DialogDescription>Radix Dialog, styled with the token system.</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="secondary">Cancel</Button>
              </DialogClose>
              <DialogClose asChild>
                <Button variant="primary">Save</Button>
              </DialogClose>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <ConfirmDialog
          trigger={
            <Button variant="danger" size="sm">
              <Trash2 size={14} strokeWidth={1.75} />
              Delete item
            </Button>
          }
          title="Delete this item?"
          description="This can't be undone."
          onConfirm={() => new Promise((resolve) => setTimeout(resolve, 600))}
        />

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="secondary" size="sm">
              Open menu
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            <DropdownMenuLabel>Actions</DropdownMenuLabel>
            <DropdownMenuItem>Edit</DropdownMenuItem>
            <DropdownMenuItem>Duplicate</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem destructive>Delete</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}
