// Catch-all sign-in route. Clerk handles every step (email, password,
// social, OTP, recovery) inside its own component.

import { SignIn } from "@clerk/nextjs";

export default function Page() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-bg px-6 py-12">
      <SignIn />
    </div>
  );
}
