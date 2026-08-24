import React from "react";

interface AuthButtonProps {
    name: string;
    call: () => void;
    icon?: string;      // URL of a brand icon, e.g. "/img/oauth/google.svg"
    disabled?: boolean;
}

// A full-width OAuth sign-in button: brand icon on the left, label centered.
// Styling lives in LoginContainer.css (.auth-button).
const AuthButton: React.FC<AuthButtonProps> = ({ name, call, icon, disabled }) => (
    <button type="button" className="auth-button" onClick={call} disabled={disabled}>
        {icon && <img className="auth-button__icon" src={icon} alt="" aria-hidden="true" />}
        <span className="auth-button__label">{name}</span>
    </button>
);

export default AuthButton;