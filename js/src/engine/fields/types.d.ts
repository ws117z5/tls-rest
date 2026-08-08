
interface NumberProps {
    value: number;
}

interface StringProps {
    value: string;
}

interface ImageProps {
    // A file <input> can't be a controlled value field, so value is optional
    value?: string;
}
 
interface ImageEditProps extends ImageProps {
    onChange: (event: React.ChangeEvent<HTMLInputElement>) => void;
}

interface NumberEditProps extends NumberProps {
    onChange: (event: React.ChangeEvent<HTMLInputElement>) => void;
} 

interface StringEditProps extends StringProps {
    onChange: (event: React.ChangeEvent<HTMLInputElement>) => void;
}

interface BooleanProps {
    value?: boolean;
    active?: boolean;
    action?: () => void;
    onChange?: (event: React.ChangeEvent<HTMLInputElement>) => void;
}
