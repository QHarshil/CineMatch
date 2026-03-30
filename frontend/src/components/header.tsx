"use client";

import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { Button } from "@/components/ui/button";

export function Header() {
  const { user, signOut, loading } = useAuth();

  return (
    <header className="border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 sticky top-0 z-50">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4">
        <div className="flex items-center gap-6">
          <Link href="/" className="text-lg font-bold tracking-tight">
            CineMatch
          </Link>
          <nav className="hidden sm:flex items-center gap-4 text-sm">
            <Link
              href="/browse"
              className="text-muted-foreground hover:text-foreground transition-colors"
            >
              Browse
            </Link>
            {user && (
              <Link
                href="/for-you"
                className="text-muted-foreground hover:text-foreground transition-colors"
              >
                For You
              </Link>
            )}
          </nav>
        </div>

        <div className="flex items-center gap-3">
          {loading ? null : user ? (
            <Button variant="ghost" size="sm" onClick={() => signOut()}>
              Sign out
            </Button>
          ) : (
            <Link href="/login">
              <Button variant="default" size="sm">
                Sign in
              </Button>
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}
