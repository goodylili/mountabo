"use client";

import { useState } from "react";
import { ChevronDown, GithubMark } from "@/components/icons";

// OwnerDropdown lets the user narrow the repo list to one owner — their account
// or any organization they have repos in — making a long list easier to search.
// Owners are derived from the fetched repos, so no extra API call is needed.
export function OwnerDropdown({
  owners,
  value,
  account,
  onChange,
}: {
  owners: string[];
  value: string | null;
  account: string | null;
  onChange: (owner: string | null) => void;
}) {
  const [open, setOpen] = useState(false);
  const label = value ?? account ?? "all owners";

  return (
    <div className="relative">
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-2 rounded-md border border-line bg-surface-2 px-2.5 py-1.5 text-cream"
      >
        <GithubMark /> {label} <ChevronDown className="text-muted" />
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute left-0 z-20 mt-1 max-h-72 w-60 overflow-y-auto rounded-lg border border-line bg-surface py-1 shadow-xl">
            <Item
              label="all owners"
              active={value === null}
              onClick={() => {
                onChange(null);
                setOpen(false);
              }}
            />
            {owners.map((o) => (
              <Item
                key={o}
                label={o}
                active={value === o}
                onClick={() => {
                  onChange(o);
                  setOpen(false);
                }}
              />
            ))}
          </div>
        </>
      )}
    </div>
  );
}

function Item({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className={`block w-full px-3 py-1.5 text-left text-[13px] transition-colors ${
        active ? "bg-lime/[0.1] text-cream" : "text-muted hover:bg-surface-hover hover:text-cream"
      }`}
    >
      {label}
    </button>
  );
}
