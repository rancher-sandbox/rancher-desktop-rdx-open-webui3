import { FadeLoader } from "react-spinners";
import './LoaderComponent.css';

export default function LoadingView() {
    return <div className="loading-container">
        <FadeLoader color="#265277" />
    </div>;
}
