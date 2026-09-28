import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MovieCard } from "./movie-card";
import { arrival, darkSeries } from "@/test/fixtures";

describe("MovieCard", () => {
  it("links to the detail page and shows year, rating, and genre", () => {
    render(<MovieCard movie={arrival} />);

    expect(screen.getByRole("link")).toHaveAttribute("href", `/movie/${arrival.id}`);
    expect(screen.getByText("Arrival")).toBeInTheDocument();
    expect(screen.getByText("2016")).toBeInTheDocument();
    expect(screen.getByText("7.6")).toBeInTheDocument();
    expect(screen.getByText("Science Fiction")).toBeInTheDocument();
    expect(screen.getByText("Film")).toBeInTheDocument();
  });

  it("labels series as TV", () => {
    render(<MovieCard movie={darkSeries} />);
    expect(screen.getByText("TV")).toBeInTheDocument();
  });

  it("shows a match badge only above 70%", () => {
    const { rerender } = render(<MovieCard movie={arrival} matchScore={0.91} />);
    expect(screen.getByText("91% match")).toBeInTheDocument();

    rerender(<MovieCard movie={arrival} matchScore={0.5} />);
    expect(screen.queryByText(/% match/)).not.toBeInTheDocument();
  });
});
