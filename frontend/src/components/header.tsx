"use client";

import Link from "next/link";
import { useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { Menu, X } from "lucide-react";
import { SearchBar } from "@/components/search-bar";

export function Header() {
  const { user, signOut, loading } = useAuth();
  const [mobileOpen, setMobileOpen] = useState(false);

  const navLinks = [
    { href: "/browse", label: "Browse" },
    ...(user ? [{ href: "/for-you", label: "For You" }] : []),
    { href: "/how-it-works", label: "How It Works" },
  ];

  return (
    <header className="fixed top-0 left-0 right-0 z-50 bg-background/80 backdrop-blur-md border-b border-border/50">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 lg:px-8">
        {/* Left: wordmark + nav */}
        <div className="flex items-center gap-8">
          <Link
            href="/"
            className="font-heading text-xl font-bold tracking-tight text-gold shrink-0"
          >
            CineMatch
          </Link>
          <nav className="hidden md:flex items-center gap-6 text-sm">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className="text-muted-foreground hover:text-foreground transition-colors duration-200"
              >
                {link.label}
              </Link>
            ))}
          </nav>
        </div>

        {/* Right: search + auth + mobile toggle */}
        <div className="flex items-center gap-4">
          <div className="hidden sm:block">
            <SearchBar variant="header" />
          </div>

          {loading ? null : user ? (
            <button
              onClick={() => signOut()}
              className="hidden md:block text-sm text-muted-foreground hover:text-foreground transition-colors duration-200 shrink-0"
            >
              Sign out
            </button>
          ) : (
            <Link
              href="/login"
              className="hidden md:block text-sm text-gold hover:text-gold-dim transition-colors duration-200 shrink-0"
            >
              Sign in
            </Link>
          )}

          <button
            className="md:hidden p-1.5 text-muted-foreground hover:text-foreground transition-colors"
            onClick={() => setMobileOpen(!mobileOpen)}
            aria-label="Toggle menu"
          >
            {mobileOpen ? (
              <X className="w-5 h-5" strokeWidth={1.5} />
            ) : (
              <Menu className="w-5 h-5" strokeWidth={1.5} />
            )}
          </button>
        </div>
      </div>

      {/* Mobile menu */}
      {mobileOpen && (
        <nav className="md:hidden bg-surface border-t border-border px-4 py-4 space-y-3">
          <div className="sm:hidden pb-2">
            <SearchBar variant="inline" />
          </div>
          {navLinks.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className="block py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
              onClick={() => setMobileOpen(false)}
            >
              {link.label}
            </Link>
          ))}
          {!loading &&
            (user ? (
              <button
                className="block w-full text-left py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
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
                className="block py-2 text-sm text-gold hover:text-gold-dim transition-colors"
                onClick={() => setMobileOpen(false)}
              >
                Sign in
              </Link>
            ))}
        </nav>
      )}
    </header>
  );
}
