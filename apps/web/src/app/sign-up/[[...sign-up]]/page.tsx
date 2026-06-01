// Catch-all sign-up route. Mirrors sign-in.

import { SignUp } from "@clerk/nextjs";

export default function Page() {
  return (
    <div className="flex min-h-screen items-center justify-center px-6 py-16">
      <SignUp />
    </div>
  );
}
