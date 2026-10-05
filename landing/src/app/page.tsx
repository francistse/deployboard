import { content } from "@/content";
import { Document } from "./document";
import { LandingView } from "./landing-view";

export default function Page() {
  return (
    <Document lang="en">
      <LandingView locale="en" content={content} />
    </Document>
  );
}
