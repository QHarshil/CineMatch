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
    { href: "/for-you", label: "For You" },
    { href: "/how-it-works", label: "How It Works" },
  ];

  return (
    <header className="fixed top-0 left-0 right-0 z-50 border-b border-border bg-background/85 backdrop-blur-md">
      <div className="mx-auto flex h-16 max-w-7xl items-stretch px-4 lg:px-8">
        {/* Wordmark */}
        <Link
          href="/"
          className="flex items-center pr-6 font-heading text-xl font-semibold uppercase tracking-tight text-primary"
        >
          CineMatch
        </Link>

        {/* Cell nav */}
        <nav className="hidden items-stretch divide-x divide-border border-x border-border md:flex">
          {navLinks.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className="eyebrow flex items-center px-6 text-muted-foreground transition-colors hover:bg-wash hover:text-primary"
            >
              {link.label}
            </Link>
          ))}
        </nav>

        {/* Search + auth */}
        <div className="ml-auto flex items-center gap-4 pl-6">
          <div className="hidden sm:block">
            <SearchBar variant="header" />
          </div>

          {loading ? null : user ? (
            <button
              onClick={() => signOut()}
              className="eyebrow hidden text-muted-foreground transition-colors hover:text-primary md:block"
            >
              Sign out
            </button>
          ) : (
            <Link
              href="/login"
              className="eyebrow hidden border border-primary px-4 py-2 text-primary transition-colors hover:bg-primary hover:text-primary-foreground md:block"
            >
              Sign in
            </Link>
          )}

          <button
            className="p-1.5 text-muted-foreground transition-colors hover:text-primary md:hidden"
            onClick={() => setMobileOpen(!mobileOpen)}
            aria-label="Toggle menu"
          >
            {mobileOpen ? (
              <X className="h-5 w-5" strokeWidth={1.5} />
            ) : (
              <Menu className="h-5 w-5" strokeWidth={1.5} />
            )}
          </button>
        </div>
      </div>

      {/* Mobile menu */}
      {mobileOpen && (
        <nav className="space-y-1 border-t border-border bg-background px-4 py-4 md:hidden">
          <div className="pb-3 sm:hidden">
            <SearchBar variant="inline" />
          </div>
          {navLinks.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className="eyebrow block py-2.5 text-muted-foreground transition-colors hover:text-primary"
              onClick={() => setMobileOpen(false)}
            >
              {link.label}
            </Link>
          ))}
          {!loading &&
            (user ? (
              <button
                className="eyebrow block w-full py-2.5 text-left text-muted-foreground transition-colors hover:text-primary"
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
                className="eyebrow block py-2.5 text-primary"
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
