<?php

namespace App\Http\Controllers\Auth;

use App\Http\Controllers\Controller;
use App\Providers\RouteServiceProvider;
use Illuminate\Auth\Events\Verified;
use Illuminate\Foundation\Auth\EmailVerificationRequest;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\JsonResponse;

class VerifyEmailController extends Controller
{
    /**
     * Mark the authenticated user's email address as verified.
     */
    public function __invoke(EmailVerificationRequest $request): JsonResponse|RedirectResponse
    {
        if ($request->user()->hasVerifiedEmail()) {
            // return redirect()->intended(
            //     config('app.frontend_url').RouteServiceProvider::HOME.'?verified=1'
            // );
            return response()->json(['redirect_to' => RouteServiceProvider::HOME]);
        }

        if ($request->user()->markEmailAsVerified()) {
            event(new Verified($request->user()));
        }

        // return redirect()->intended(
        //     config('app.frontend_url').RouteServiceProvider::HOME.'?verified=1'
        // );
        return response()->json(['redirect_to' => RouteServiceProvider::HOME]);
    }
}
