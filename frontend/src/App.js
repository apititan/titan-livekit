import React, {  useEffect } from "react";
import "./header.css"
import {
    BrowserRouter as Router,
    Switch,
    Route,
    Link,
    Redirect
} from "react-router-dom";
import ChatList from "./ChatList";
import Login from "./Login";
import {connect} from 'react-redux'
import {clearRedirect, setProfile, unsetProfile} from "./actions";
import axios from 'axios'
import Chat from './Chat'

/**
 * Main landing page user starts interaction from
 * @returns {*}
 * @constructor
 */

const App = ({ currentState, dispatch }) => {

    // https://ru.reactjs.org/docs/hooks-effect.html
    useEffect(() => {
        function showHeader() {
            console.log("Called showHeader");
            var header = document.querySelector('.header');
            if(window.pageYOffset > 200){
                header.classList.add('header_fixed');
            } else{
                header.classList.remove('header_fixed');
            }
        }
        console.log("Add header scroll listener");
        window.onscroll = showHeader;

        return function cleanup() {
            console.log("Removing header scroll listener");
            window.onscroll = null;
        };
    });

    function redirector() {
        // https://tylermcginnis.com/react-router-programmatically-navigate/
        // https://medium.com/@anneeb/redirecting-in-react-4de5e517354a
        const re = currentState.redirectUrl;
        if (re) {
            console.log("Performing redirect to", re);
            // workaround https://github.com/facebook/react/issues/18178#issuecomment-595846312
            setTimeout(()=>dispatch(clearRedirect()), 0);
            return <Redirect to={re} />
        }
    }

    function logout() {
        axios.post(`/api/logout`)
            .then(value1 => {
                return dispatch(unsetProfile())
            })
    }

    return (
        <Router>
        <header className="header">
            <div className="wrapper">
                <div className="row">
                    <div className="logo">
                        <img src="/public/assets/pandas.jpg"
                             alt="Logo" className="logo__pic"/>
                    </div>
                    <nav className="menu">
                        <Link className="menu__item" to="/">Главная</Link>
                        <Link className="menu__item" to="/about">About</Link>
                        <Link className="menu__item" to="/chat">Chats</Link>
                    </nav>
                    { currentState.profile ?
                        (<div>
                            <div>{ currentState.profile.login} </div>
                            <div className="logout">
                                <a className="menu__item login__btn" onClick={() => logout()}>Выйти</a>
                            </div>
                        </div>) :
                        (<div className="login">
                            <Link className="menu__item login__btn" to="/login">Войти</Link>
                        </div>)
                    }
                </div>
            </div>
        </header>

        { redirector() }

        {/* A <Switch> looks through its children <Route>s and
        renders the first one that matches the current URL. */}
        <Switch>
            <Route path="/login">
                <Login />
            </Route>
            <Route path="/about">
                <About />
            </Route>
            <Route exact path="/chat">
                <ChatList />
            </Route>
            <Route path="/chat/:id">
                <Chat />
            </Route>
            <Route path="/">
                <Home />
            </Route>

        </Switch>

        </Router>
    );
};

function Home() {
    return <div>
        <h2>Welcome</h2>
        <div>Best place to chatting with your friends.</div>
    </div>;
}

function About() {
    return <div>
        This is best application for video calls and text messaging. We was hardworking.
    </div>;
}

const mapStateToProps = state => ({
    currentState: state
});

// https://codesandbox.io/s/github/reduxjs/redux/tree/master/examples/todos?from-embed=&file=/src/containers/VisibleTodoList.js
// https://react-redux.js.org/using-react-redux/connect-mapstate
// https://habr.com/ru/company/ruvds/blog/423157/
export default connect(
    mapStateToProps
)(App)