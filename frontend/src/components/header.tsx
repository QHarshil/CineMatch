"use client";

import Link from "next/link";
import { useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { Button } from "@/components/ui/button";

export function Header() {
  const { user, signOut, loading } = useAuth();
  const [mobileOpen, setMobileOpen] = useState(false);

  const navLinks = [
    { href: "/browse", label: "Browse" },
    ...(user ? [{ href: "/for-you", label: "For You" }] : []),
  ];

  return (
    <header className="border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 sticky top-0 z-50">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4">
        {/* Left: logo + desktop nav */}
        <div className="flex items-center gap-6">
          <Link href="/" className="text-lg font-bold tracking-tight">
            CineMatch
          </Link>
          <nav className="hidden sm:flex items-center gap-4 text-sm">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className="text-muted-foreground hover:text-foreground transition-colors"
              >
                {link.label}
              </Link>
            ))}
          </nav>
        </div>

        {/* Right: auth + mobile hamburger */}
        <div className="flex items-center gap-3">
          {loading ? null : user ? (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => signOut()}
              className="hidden sm:inline-flex"
            >
              Sign out
            </Button>
          ) : (
            <Link href="/login" className="hidden sm:inline-flex">
              <Button variant="default" size="sm">
                Sign in
              </Button>
            </Link>
          )}

          {/* Mobile hamburger */}
          <button
            className="sm:hidden p-2 text-muted-foreground hover:text-foreground"
            onClick={() => setMobileOpen(!mobileOpen)}
            aria-label="Toggle menu"
          >
            {mobileOpen ? (
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            ) : (
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M3 12h18M3 6h18M3 18h18" />
              </svg>
            )}
          </button>
        </div>
      </div>

      {/* Mobile menu */}
      {mobileOpen && (
        <nav className="sm:hidden border-t border-border bg-background px-4 py-3 space-y-2">
          {navLinks.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className="block py-2 text-sm text-muted-foreground hover:text-foreground"
              onClick={() => setMobileOpen(false)}
            >
              {link.label}
            </Link>
          ))}
          {!loading && (
            user ? (
              <button
                className="block w-full text-left py-2 text-sm text-muted-foreground hover:text-foreground"
                onClick={() => {
                  signOut();
                  setMobileOpen(false);
                }}
              >
                Sign out
              </button>
            ) : (
              <Link
                href="/login"
                className="block py-2 text-sm text-muted-foreground hover:text-foreground"
                onClick={() => setMobileOpen(false)}
              >
                Sign in
              </Link>
            )
          )}
        </nav>
      )}
    </header>
  );
}
