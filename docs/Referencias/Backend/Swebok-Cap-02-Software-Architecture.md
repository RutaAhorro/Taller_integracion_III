## CHAPTER 02

## Software Architecture

## ACRONYMS

| AD   | Architecture Description                |
|------|-----------------------------------------|
| ADL  | Architecture Description Language       |
| API  | Application Programming Interface       |
| ASR  | Architecturally Significant Requirement |
| ATAM | Architectural Tradeoff Analysis Method  |
| IDL  | Interface Description Language          |
| MVC  | Model View Controller                   |
| QAW  | Quality Attribute Workshop              |
| RA   | Reference Architecture                  |
| REST | Representational State Transfer         |
| SAAM | Software Architecture Analysis Method   |
| UML  | Unified Modeling Language               |

## INTRODUCTION

## 1. Software Architecture Fundamentals [2*, c1] [29*, appendix C] [38*, c2] [41*, c1-3]

<!-- formula-not-decoded -->

Software engineering and related disciplines use many senses of "architecture." First, "architecture" often refers to a discipline: the art and science of constructing things — in this case, software-intensive systems. The discipline involves concepts, principles, processes and methods the community has discovered and adopted.

Second, "architecture" refers to the various processes through which that discipline is realized. Software architecture is also considered part of Software Design; generally considered a multistage process, divided into the following stages:

- Architectural design stage
- High-level design stage

This chapter considers software architecture from several perspectives: concepts; representation and work products; context, process and methods; and analysis and evaluation.

- Detailed design stage

Software design is the focus of Chapter 3. This chapter focuses on architecting and architectural design.

In contrast to the previous edition, this edition creates a software architecture knowledge area (KA), separate from the Software Design KA, because of the significant interest and growth of the discipline since the 1990s.

## BREAKDOWN OFTOPICS FOR SOFTWARE ARCHITECTURE

The breakdown of topics for the Software Architecture KA is shown in Figure 2.1.

Third, "architecture" refers to the outcome of applying architectural design discipline and processes to devise architectures for software systems. Architectures as outcomes are expressed in architecture descriptions. This is discussed in topic Software Architecture Description. The concept of architecture has evolved, and many definitions are in use today. One early definition of architecture, from 1990, emphasized software structure:

Architecture. The organizational structure of a system or component. [from: IEEE

Figure 2.1. Breakdown of Topics for the Software Architecture KA

<!-- image -->

Std 610.12-1990, IEEE Glossary of Software of? What interactions does it have with other Engineering Terminology] systems? How are properties like safety and security handled? The recognition that software contains many different structures has prompted discussion of a number of interesting concepts about software architecture

This definition did not do justice to evolving thinking about architecture; e.g., this definition does not allow us to distinguish the detailed design of a module from its Makefile. Either Sa (i o  a nn  as ze a s ing the software system or component but should to current definitions such as: not be considered architecture. Moreover, emphasis on the structure was often limited to the code's structure and failed to encompass all the structures of the software system:

The software architecture of a system is the set of structures needed to reason about the system. These structures comprise software eleof both. [2*]

architecture (of a system). fundamental concepts or properties of a system in its environment embodied in its elements, relationships, and in the principles of its design and evolution [23]

Key ideas in that definition are the folments, relations among them, and properties lowing: (1) Architecture is about what is fundamental to a software system; not every element, interconnection, or interface is conDuring the mid-1990s, however, software sidered fundamental. (2) Architecture conarchitecture emerged as a broader discipline siders a system in its environment. Much like involving a more generic study of software building architecture, software architecstructures and architectures. Many software ture is outward-looking; it considers a syssystem structures are not directly reflected tem's context beyond its boundaries including in the code structure. Both types of structhe people, organizations, software, hardture have implications for the system as a ware and other devices with which the system whole: What behaviors is the system capable must interact.

<!-- formula-not-decoded -->

A software system has many stakeholders with varying roles and interests relative to that system. These varying interests are termed concerns, following Dijkstra's separation of concerns:

Let me try to explain to you, what to my taste is characteristic for all intelligent thinking. It is, that one is willing to study in depth an aspect of one's subject matter in isolation for the sake of its own consistency, all the time knowing that one is occupying oneself only with one of the aspects. We know that a program must be correct and we can study it from that viewpoint only; we also know that it should be efficient and we can study its efficiency on another day, so to speak. In another mood we may ask ourselves whether, and if so: why, the program is desirable. But nothing is gained — on the contrary! — by tackling these various aspects simultaneously. It is what I sometimes have called "the separation of concerns", which, even if not perfectly possible, is yet the only available technique for effective ordering of one's thoughts, that I know of. This is what I mean by "[focusing] one's attention upon some aspect": it does not mean ignoring the other aspects, it is just doing justice to the fact that from this aspect's point of view, the other is irrelevant. It is being one- and multiple-track-minded simultaneously. [12]

What is fundamental about a system varies according to stakeholders' concerns and roles. The software structures, therefore, also vary with stakeholder roles and concerns. (See also topic Design Methods in Software Design KA.)

A software system's customer is most interested in when the system will be ready and how much it will cost to build and operate. Ue a s  a   aos nd how to use it. Designers and programmers building the system have their own concerns, such as whether an algorithm will meet the system requirements. Those responsible for ensuring the system is safe to operate have different concerns.

affordability, agility, assurance, autonomy, availability, behavior, business goals and strategies, complexity, compliance with regulation, concurrency, control, cost, data accessibility, deployability, disposability, energy efficiency, evolvability, extensibility, feasibility, flexibility, functionality, information assurance, inter-process communication, interoperability, known limitations, maintainability, modifiability, modularity, openness, performance, privacy, quality of service, reliability, resource utilization, reusability, safety, scalability, schedule, security, system modes, software structure, subsystem integration, sustainability, system features, testability, usability, usage, user experience affordability, agility, assurance, autonomy, availability, behavior, business goals and strategies, complexity, compliance with regulation, concurrency, control, cost, data accessibility, deployability, disposability, energy efficiency, evolvability, extensibility, feasibility, flexibility, functionality, information assurance, inter-process communication, interoperability, known limitations, maintainability, modifiability, modularity, openness, performance, privacy, quality of service, reliability, resource utilization, reusability, safety, scalability, schedule, security, system modes, software structure, subsystem integration, sustainability, system features, testability, usability, usage, user experience

Figure 2.2. Examples of Architectural Concerns Figure 2.2. Examples of Architectural 1 Concerns

Concerns encompass a broad range of issues, possibly pertaining to any influence on a system in its environment, including developmental, technological, business, operational, organizational, political, economic, legal, regulatory, ecological and social influences. Like software requirements, they may be classified as functional, non-functional or constraint. (See Software Requirements KA.) Concerns manifest in various familiar forms, including requirements, quality attributes or "ilities", emergent properties (which may be either desired or prohibited) and various kinds of constraints (as listed above). See Software Quality KA. Topic 2, Software Architecture Description, shows how concerns shape architecture and the work products describing those architectures. Examples of concerns are depicted in Figure 2.2. Concerns are not static; concerns evolve over the life cycle of a system and as technologies, policies and other influences evolve. For example, due to increased awareness of climate change, there is growing interest in concerns such as energy efficiency, and sustainability [24].

1.3. Uses of Architecture fundamental concepts or properties of a soft[2*, c24] [38*, c30] ware system in its environment. But each stakeholder can have a different notion of A principal use of a software system's archi- what is fundamental to that software system, tecture is to give those working with it a given their perspective. Having a mental shared understanding of the system to guide model of a system's architecture is perhaps its design and construction. An architecy      s ture also serves as a preliminary conception working alone. However, for large, complex of the software system that provides a basis systems developed and operated by teams, a to analyze and evaluate alternatives. A third tangible representation is invaluable, especommon usage is to enable reverse engicially as the conception of the system evolves, neering (or reverse architecting) by helping and as people join or leave the team. Having a those working with it to understand an concrete representation as a work product can existing software system before undertaking also serve as a basis to analyze the architecmaintenance, enhancement or modification. ture, organize its design and guide its impleTo support these uses, the architecture should mentation. These work products are called be documented (see topic Software Architecture architecture descriptions (ADs). Description).

Historically, ADs used text and informal diagrams to convey the architecture. However, the diversity of stakeholder audiences and their different concerns have led to a diversity of representations of the architecture. Notations should be chosen based on the Often, these representations are specialized based upon existing practices of the com2. Software Architecture Description munities or disciplines involved to effectively [2*, c1.2, 22] [38*, c12-13] [40*, c6] address this variety of stakeholders and con[41*, c6-7] cerns (see Software Design KA and Software Engineering Models and Methods KA). In topic 1, Software Architecture Fundamentals, These various representations are called archia software architecture was defined as the tecture views.

ADs document an architecture for a softConway's Law posits that "organizations ware system. It is targeted to those stakewhich design systems ... are constrained to holders of the system who have concerns about produce designs which are copies of the comthe software system which are answered by munication structures of these organizations" the architecture. As noted in topic 1, Software [11]. Empirical studies have observed that the Architecture Fundamentals, a primary audiarchitectures of these systems often mirror the ence comprises the designers, engineers and communications structures of those organizaprogrammers whose concerns pertain to contions [28]. Depending on the software system structing the system. For these stakeholders, and the organization, this can be a strength ADs serve as a blueprint to guide the construcor a weakness. The architecture can enhance tion of the software system. For others, the communication within a large team or comAD is a basis for their work — for example, promise it. Each part of the organization can testing and quality assurance, certification, base its planning, costing and scheduling deployment, operation, and maintenance and activities upon its knowledge of the architecfuture evolution. ture. Creating a well-planned and documented architecture is one approach to increasing the applicability and reusability of software designs and components. The architecture forms the basis for design families of programs or software product lines. This can be done by identifying commonalities among members of need, purpose and the utility of those choices such families and by designing reusable and (such as understandability, familiarity) for customizable components to account for the the stakeholders who need that information. variability among family members.

## 2.1. Architecture Views and Viewpoints

An architecture view represents one or more aspects of an architecture to address one or Each viewpoint provides a vocabulary or more concerns [38*]. Views address distinct language for talking about a set of concerns concerns — for example, a logical view (depicts and the mechanisms for addressing them. how the system will satisfy the functional The viewpoint language gives stakeholders requirements); a process view (depicts how the a shared means of expression. Viewpoints system will use concurrency); a physical view need not be limited to one software system (depicts how the system is to be deployed and but are reusable by an organization or applidistributed) and a development view (depicts cation community for many similar systems. how the top-level design is broken down When generic representations such as Unified into implementation units, the dependencies Modeling Language (UML) are used, they among those units and how the implementacan be specialized to the system, its domain tion is to be constructed). Separating concerns or the organizations involved. (See section by view allows interested stakeholders to focus 2.3 Architecture Description Languages and on a few things at a time and offers a means of Architecture Frameworks.) managing the architecture's understandability and overall complexity.

Other documented viewpoints include view[6*, c7-9] [29*, c8] [38*, c3] [40*, c6.2] points for availability, behavior, communications, exception handling, performance, reliability, safety and security.

Architecture practice has evolved from the use of text and informal diagrams to the use of more rigorous representations. Each architecture view depicts architectural elements of the system using well-defined conventions, notations and models [38*]. The conventions for each view are documented as an architecture viewpoint [23]. Viewpoints guide the creation, interpretation and uses of architecture views. Each viewpoint links stakeholder audience concerns with a set of conventions. In software issues. Clements et al. have intromodel-based architecting, each view can be duced viewtypes which establish a 3-way catmachine-checked against its viewpoint.

Common viewpoints include the module viewpoint, used to express a software system's implementation in terms of its modules and Architecture descriptions frequently use their organization [2*]; the component and multiple architecture views to represent the connector viewpoint, used to express the softdiverse structures needed to address different ware's large-scale runtime organization and stakeholders' various concerns. There are two interactions [2*]; the logical viewpoint, used common approaches to the construction of to express fundamental concepts of the softviews: the synthetic approach and the projective ware's domain and capability [25]; the sceapproach. In the synthetic approach, architects narios/use cases viewpoint, used to express construct views of the system-of-interest and how users interact with the system [25]; the integrate these views within an architecture information viewpoint, used to express a sysdescription using correspondence rules. In tem's key information elements and how they the projective approach, an architect derives are accessed and stored [38*]; and the deployeach view through some routine, possibly ment viewpoint, used to express how a system mechanical, procedure of extraction from a is configured and deployed for operation [38*]. single unified model (or "uber model") [23].

Beyond specifying forms of representation, an architecture viewpoint can capture the ways of working within a discipline or community of practice. For example, a software reliability viewpoint captures existing practices from the software reliability community for identifying and analyzing reliability issues, formulating alternatives and synthesizing and representing solutions. Like engineering handbooks, general-purpose and specialized viewpoints provide a means to document repeatable or reusable approaches to recurring egorization of viewpoints. These categories are module, component and connector, and allocation viewtypes [9].

A consequence of introducing multiple views into an AD is a potential mismatch between the views. Are they consistent? Are they describing the same system? This has been called the multiple views problem [39]. The projective approach limits possible inconsistencies, since views are derived from a single (presumably consistent) model, but at the cost of expressiveness: the underlying model may not be capable of capturing arbitrary concerns. Under the synthetic approach, architects integrate views into a whole, using linkages or other forms of traceability to cross-reference view elements to achieve consistency [23,25]. Viewpoints often include rules for establishing consistency or other relationships among views.

- Adaptive systems (e.g., microkernel, reflection and meta-level architectures)
- Virtual machines (e.g., interpreters, rulebased, process control)

Pattern catalogs (or systems of patterns) are used to express architectural styles and solutions through coordinated sets of patterns. Examples of pattern catalogs are [7], [19] for n-tier architectures, [13] for service-oriented architecture and [37] for microservice architectures. Pattern catalogs are not limited to architecture styles and can be focused on addressing specific concerns, such as security [17].

<!-- formula-not-decoded -->

Inspired by its use in the long history of the architecture of buildings, an architectural style is a particular manner of construction yielding a software system's characteristic features. An architectural style often expresses a software system's large-scale organization. In contrast, an architectural pattern expresses a common solution to a recurring problem within the context of a software system — it need not apply to the whole system. Design patterns are discussed in section 4.4 of Software Design KA.

There is no strict dividing line between s architectural styles and patterns. Both patterns and styles provide solutions to specific problems in given contexts. An architectural style expresses the global aspects of a system or subsystem by defining its major parts of that (sub)system and how they interact [7,38*]. An architectural style can be expressed as an architectural pattern [7]. Architectural patterns exist at varying scales and could apply once to a single element of a system or be applied repeatedly throughout a system.

Various architectural styles and patterns have been documented [7,39]:

- General structures (e.g., layered, calland-return, pipes and filters, blackboard, services and microservices)
- Distributed systems (e.g., client-server, n-tier,broker, publish-subscribe, point-topoint, representational state transfer (REST))

In relation to architecture viewpoints, which provide the languages for talking about various aspects of software systems, a unifying notion is that both patterns and styles are idioms in those languages for expressing particular aspects of architectures (and designs, see section 4.4 Design Patterns in Software Design KA). An architectural pattern or style uses a vocabulary, drawn from the viewpoint's language, in a specified way, to talk about view elements, including element and relation types and their instances, and constraints on combining them [23,39]. In this way, viewpoints, patterns and styles are mechanisms for codifying recommended practices to facilitate reuse.

- Method-driven (e.g., object-oriented, event-driven, data flow)
- User-computer interaction (e.g., modelview-controller, presentation-abstractioncontrol)

A reference architecture (RA) is an architecture constraining or guiding other architectures. Documented as a reference architecture description, an RA provides a common basis for the development of architectures for individual systems, product lines or families of systems and application domains. Reference architectures capture commonalities to promote ease of development, integration software system, its requirements, and the and interoperability and other kinds of stanavailable resources during development and dardization. Reference architectures have throughout the life cycle. The impact on been developed and used in many domains quality attributes and trade-offs among including automotive systems, healthcare, competing quality attributes are often the Internet of'Things, cloud computing, avionics, basis for design decisions. manufacturing and telecommunications.

<!-- formula-not-decoded -->

An architecture description language (ADL) is a domain-specific language for expressing software architectures. ADLs arose from module interconnection languages [36] for Architecture rationale captures why an archiprogramming in the large. Some ADLs target tectural decision was made. This includes a single application domain or architectural assumptions made before the decision, alterstyle (such as MetaH for avionics systems in an natives considered, and trade-offs or criteria event-driven style), others are wide spectrum used to select an approach and reject others. to frame concerns across the enterprise (such as Recording rejected decisions and the reasons ArchiMate™). UML has frequently been used for their rejection can also be useful. In the as an ADL due to its widespread use in soft- future, this could either prevent a software ware design activities [41*]. ADLs often proproject from making a poor decision — one vide capabilities beyond description to enable rejected earlier for forgotten reasons — or architecture analysis or code generation. allow the development to recognize that relAn architecture framework captures the evant conditions have changed and that they can revisit the decision.

The architectural design activity creates a network of decisions as its outcome, with some decisions deriving from prior decisions. [2*, c22] Decisions can be explicitly documented, along with an explanation of the rationale for each nontrivial decision. Decision analysis provides one approach to architecture evaluation. (See topic 4, Software Architecture Evaluation.)

"conventions, principles and practices for the description of architectures established within Architectural technical debt has been introa specific domain of application and/or comduced to reflect that today's decisions for munity of stakeholders" [23]. Frameworks an architecture may have significant concodify recommended practices within a spesequences later in the software system's life cific domain and are implemented as an intercycle. Decisions deferred can compromise its locking set of viewpoints or ADLs. Examples maintainability or the future evolvability, and are AUTOSAR for the automotive industry, that debt will have to be paid — typically by OMG's Unified Architecture Framework others, not necessarily by those who caused (UAF®) and ISO Reference Model for Open the debt. Such debt has an economic impact Distributed Processing. on the system's future development and operations. For example, when a software project 2.4. Architecture as Significant Decisions has limited time, it may develop an initial [38*, c8] [40*, c6.1] design with little concern for modularity for its first release. The lack of modularity can Architectural design is a creative process. adversely affect the development time for subDuring this activity, architects make many sequent releases, impact developers, and perdecisions that profoundly affect the archihaps compromise future maintainability of tecture, the downstream development prothe system. Additional functionality can be cess and the software system. Many factors added later only by doing extensive refactoring affect decision-making, including promwhich impacts future timelines and introinent concerns of stakeholders for the duces additional defects. [26]. Architectural technical debt can be analyzed and managed, architecture, while other requirements are like other concerns, using models and viewdeferred to subsequent stages of the software points [27]. process, such as design or construction.

Figure 2.3. A general model of architectural design

<!-- image -->

## 3. Software Architecture Process

This section outlines a general model of an architectural design process. It is used to developed against specific product requiredemonstrate how architectural design fits into the general context of software engineering processes (see Software Engineering Process KA) and as a framework for understanding the description might be the code itself. In some many architecture methods currently in use. It agile practices, the software architecture is said also recognizes that architectural design can to "emerge" from coding the system based on take place in a variety of contexts.

In product line or product family settings, a product line/family architecture is devel[29*, c9] [38*, c6-7] [41*, c4] oped against a basic set of needs, requirements and other factors. That architecture will be the starting point for one or more product instances ments, building upon the product baseline.

In agile approaches, there is not usually an architecture design stage. The only architecture user stories through a rapid series of development cycles. Although this approach has had 3.1. Architecture in Context some success with user-centric information sys[29*, c12-13] [41*, c2] tems, it is difficult to ensure an adequate architecture emerges for other classes of applications, Architecture occurs in several contexts. In such as embedded and cyber-physical systems, not be articulated by any user stories.

the traditional life cycle, there is an architec- when critical architectural properties might tural design stage driven by software system requirements (see Software Requirements In enterprise and system-of-systems conKA). Some requirements will be architectural texts, as in product lines and families, the drivers, influencing major decisions about the overarching architecture (of the enterprise, system or product line/family) provides primary requirements and guidance on the form and constraints upon the software architecture. This baseline can be enforced through specifications, additional requirements, application programming interfaces (APIs) or conformance suites.

<!-- formula-not-decoded -->

- Large-scale refinement of the system into key components
- Communication and interaction among components
- Allocation of concerns and design responsibilities to components
- Component interfaces
- Understanding and analysis of scaling and performance properties, resource consumption properties, and reliability properties

Design and architecture are often blurred. It Large-scale/system-wide approaches to has been said that architecture is the set of dominating concerns (such as safety and decisions that one cannot trust to designers. security, where applicable) In fact, architecture emerged out of software design as the discipline matured, largely since An overview of architectural design is prethe 1990s. There are various contrasts: design sented in Figure 2.3. often focuses on an established set of requireArchitectural design is iterative, comments, whereas architecture often must shape prising three major activities: analysis, synthe requirements through negotiation with thesis and evaluation. Often, all three major stakeholders and requirements analysis. In activities are performed concurrently at varaddition, architecture often must recognize ious levels of granularity. and address a wider range of concerns that [3 ] ] r iss      n  t   ss software system of interest.

Architecture analysis gathers and formulates 3.2. Architectural Design [2*, c19-23] architecturally significant requirements (ASRs), defined as any "requirement upon a software Architectural design is the application of system which influences its architecture" [31]. design principles and methods within a Architecture analysis is based on identified process to create and document a software concerns and on understanding the software's architecture. There are many architecture context, including known requirements, methods for carrying out this activity. This stakeholder needs and the environment's consection describes a general model of architecstraints. ASRs reflect the design problems tural design underlying various architecture the architecture must solve. Often the commethods based upon [20]. bination of initial requirements and known Architectural design involves identifying a constraints cannot be satisfied without consesystem's major components; their responsibilquences to cost, schedule, etc. In such cases, ities, properties, and interfaces; and the relanegotiation is used to modify incoming needs, tionships and interactions among them and requirements and expectations to make soluwith the environment. In architectural design, tions possible. Architecture analysis produces fundamentals of the system are decided, but ASRs, initial system-wide decisions and any other aspects, such as the internal details of overarching system principles derived from major components are deferred. the context (see Architecture in Context).

Typical concerns in architectural design include the following:

## 3.2.2. Architecture Synthesis

[2*, c20]

ya s sss rse c  ss r aa puting paradigms solutions in response to the outcomes of architecture analysis. Synthesis proceeds by working out detailed solutions to design problems identified by ASRs, and makes tradeoffs to accommodate interactions between those solutions. These outcomes feed back to architecture analysis resulting in elaborated ASRs, principles and decisions which then lead to further detailed solution elements.

<!-- formula-not-decoded -->

- that implementations conform to the architecture
- architecture maintenance: managing and extending the architecture following its implementation
- architecture management: managing an organization's portfolio of interrelated architectures
- architecture knowledge management: extracting, maintaining, sharing and exploiting reusable architecture assets, including decisions, lessons learned, specifications and documentation across the organization

## e 4. Software Architecture Evaluation

[2*, c21] [38*, c14] [41*, c8]

<!-- formula-not-decoded -->

Architecture evaluation validates whether the chosen solutions satisfy ASRs and when and where rework is needed. Architecture evaluation methods are discussed in topic 4 Software Architecture Evaluation.

<!-- formula-not-decoded -->

There are a number of documented architec- Architecture analysis takes place throughout

ture methods (see Further Readings for a list). the process of creating and sustaining an architecture. Architecture evaluation is typ3.4. Architecting in the Large ically undertaken by third parties at deter[29*, c12, 14] [40*, c19] mined milestones as a form of assessment.

Given the multi-concern, multi-disciArchitectural design denotes a specific stage plinary nature of software architecture, there of the life cycle, but is only one part of softare many aspects to what makes an architecture ware architecting. Software architecting does "good." The Roman architect Vitruvius posited not occur in a vacuum, as noted in section 3.1 that all buildings should have the attributes of Architecture in Context, but in an environment frmitas, utilitas and venustas (translated from that often includes other architectures. For Latin as strength, utility and beauty). example, an application architecture should Of a software system and its architecture, conform to an enterprise architecture; to "play one can ask: well" in a system of systems, the architecture of each constituent system should conform to the · Is it robust over its lifetime and possible system of systems architecture. In such cases, evolution? these relations need to be reflected as ASRs on · Is it fit for its intended use? the software being architected. Many software · Is it feasible and cost-effective to construct architecting activities and principles are not software systems using this architecture? limited to software but equally apply to systems · Is it, if not beautiful, then at least clear and enterprise architecting [29]. Weinreich and understandable to those who must and Buchgeher have extended Hofmeister construct, use and maintain the software? et al.'s model used in section 3.2 Architectural Design to include these activities [42]:

Each architecture concern may be a basis for evaluation. Evaluation is conducted against · architecture i implementation: over-requirements (when available) or against need, srpts   g  stng enn ng etns enns A "good" architecture should address not only use case to the software architecture elements the distinct concerns of its stakeholders, but that would be involved in carrying out those also the consequences of their interactions. steps [23]. For example, a secure architecture may be For a general framework for reasoning excessively costly to build and verify; an easyabout various concerns, see Bass et al. [3]. to-build architecture may not be maintainable over the system's lifetime if it cannot incorpo4.3. Architecture Reviews [2*, c21] rate new technologies.

The Architecture Tradeoff Analysis Architecture reviews are an effective approach Method (ATAM) [10] provides a method- to assess an architecture's status and quality and identify risks by assessing one or more architecture concerns [1]. Many reviews are informal or expertise-based, and some are more structured, organized around a checklist of topics to cover. Parnas and Weiss proposed an effective approach to conducting reviews, called active reviews [33], where instead of checklists, each evaluation item entails a specific activity by a reviewer to obtain the needed information.

ical approach to evaluating software architectures based on quality attributes in a utility tree and scenarios illustrating the qualities. Analysis of tradeoffs among competing quality requirements and their architectural approaches are the key to the architecture evaluation. Clements, et al. describe several methods for evaluation including ATAM, Software Architecture Analysis Method (SAAM), and Quality Attribute Workshops (QAW) [10]. The SARA Report defines a general framework for software architecture evaluation [31].

## 4.2. Reasoning about Architectures

Many organizations have institutionalized architecture review practices. For example, an industry group developed a framework for defining, conducting and documenting architecture reviews and their [38*, c10] outcomes [31].

Each architecture concern has a distinct basis 4.4. Architecture Metrics [2*, c23] for evaluation. Evaluation is most effective when it is based upon robust, existing archiAn architecture metric is a quantitative meatecture descriptions. ADs can be queried, sure of a characteristic of an architecture. examined and analyzed. For example, evalVarious architecture metrics have been uation of functionality or behavior benefits defined. Many of these originated as design or from having an explicit architecture view code metrics that have been "lifted" to apply or other representation of that aspect of the to architecture. Metrics include component system to study. Specialized concerns such as dependency, cyclicity and cyclomatic comreliability, safety and security often rely on plexity, internal module complexity, module specialized representations from the respeccoupling and cohesion, levels of nesting, and tive discipline. compliance with the use of patterns, styles

Often architecture documentation is unfin- and (required) APIs. ished, incomplete, out of date or nonexistent. In such cases, the evaluation effort must rely (such as DevOps), other metrics have evolved on the knowledge of participants as a primary information source.

In continuous development paradigms that focus not on the architecture directly but on the responsiveness of the process, such as Use cases are frequently used to check metrics for lead time for changes, deployment

an architecture's completeness and consis- frequency, mean time to restore service, and tency (see Software Engineering Models and change failure rate — as indicative of the state Methods KA) by comparing the steps in the of the architecture.

## MATRIX OFTOPICS VS. REFERENCE MATERIAL

|                                                                     | Bass et al. [2*]   | Budgen [6*]   | Maier et al. [29*]   | Rozanski et al. [38*]   | Sommerville [40*]   | Taylor et al. [41*]   |
|---------------------------------------------------------------------|--------------------|---------------|----------------------|-------------------------|---------------------|-----------------------|
| 1. Software Architecture Fundamentals                               | c1                 |               | Appendix C           | c2                      |                     | c1-3                  |
| 1.1. The Senses of "Architecture"                                   | c2                 | c6.1          | c6                   |                         |                     |                       |
| 1.2. Stakeholders and Concerns                                      | c3-14              |               |                      | c8-9                    |                     | c3                    |
| 1.3. Uses of Architecture                                           | c24                |               |                      | c30                     |                     |                       |
| 2. Software Architecture Description                                | c1.2,22            |               |                      | c12-13                  | c6                  | c6-7                  |
| 2.1. Architecture Views and Viewpoints                              |                    | c7-9          | c8                   | c3                      | c6.2                |                       |
| 2.2. Architecture Patterns, Styles and Reference Architectures      | c2.12              | c6,15         |                      | c11                     | c6.3                | c11                   |
| 2.3. Architecture Description Languages and Árchitecture Frameworks | c22                |               | c11                  | app                     |                     | c6-7                  |
| 2.4. Architecture as Signifcant Decisions                           |                    |               |                      | c8                      | c6.1                |                       |
| 3. Software Architecture Process                                    |                    |               | c12-13               | c6-7                    |                     | c4                    |
| 3.1. Architecture in Context                                        |                    |               | c12-13               |                         |                     | c2                    |
| 3.1.1. Relation of Architecture to Design                           |                    |               |                      |                         | c6                  | c2                    |
| 3.2. Architectural Design                                           | c19-23             |               |                      |                         |                     |                       |
| 3.2.1. Architecture Analysis                                        | c19                |               |                      |                         |                     | c8                    |
| 3.2.2. Architecture Synthesis                                       | c20                |               |                      |                         |                     |                       |
| 3.2.3. Architecture Evaluation                                      | c21                |               |                      | c14                     |                     |                       |
| 3.3. Architecture Practices, Methods, and Tactics                   | c3.4               |               | c10                  | c9-14                   |                     |                       |

| 3.4. Architecting in the Large      |        |     | c12,14   |     | c19   |    |
|-------------------------------------|--------|-----|----------|-----|-------|----|
| 4. Software Architecture Evaluation | c21    |     |          | c14 |       | c8 |
| 4.1. "Goodness" in Architecture     | c1.3,2 | c17 |          |     |       |    |
| 4.2. Reasoning about Architectures  |        |     |          | c10 |       |    |
| 4.3 Architecture Reviews            | c21    |     |          |     |       |    |
| 4.4 Architecture Metrics            | c23    |     |          |     |       |    |

## FURTHER READINGS

Perry and Wolf, Foundations for the study of software architecture [34]

Perry and Wolf's Foundations circulated informally for several years before its publication in 1992. It has indeed served as a foundation for the evolution of the discipline of software architecture, introducing a number of ideas that are fundamental to the field, including architecture as a discipline; distinguishing architecture and design; elements of software architectures; multiple views; architecture styles and types; and analogies with other fields.

reflecting both the opportunities and constraints that organizations encounter.

Kruchten, The 4+1 View Model of Architecture [25].

This seminal paper organizes an approach to architecture description using five architecture viewpoints. The first four are used to produce the logical view, the development view, the process view, and the physical view. These are integrated through selected use cases or scenarios to illustrate the architecture. Hence, the model results in 4+1 views. The views are used to describe the software as envisioned by different stakeholders — such as end-users,

Bass et al., Software Architecture in Practice [2*] developers, and project managers.

This book introduces concepts and recom- Rozanski and Woods, Software Systems Architecture [38*]

mended practices of software architecture, meaning how software is structured and how the software's components interact. The book addresses several quality concerns in detail, including: availability, deployability, energy efficiency, modifiability, performance, testability and usability. The authors offer recommended practices focusing on architectural design, architecture description, architecture evaluation and managing architecture technical debt. They also emphasize the importance of the business context in which large software is designed. In doing so, they present software architecture in a real-world setting, This is a handbook for the software systems architect. It develops key concepts of stakeholder, concern, architecture description, architecture viewpoint and architecture view, architecture patterns and styles, with examples. It provides an end-to-end architecting process. The authors provide a catalog of ready-to-use, practical viewpoints for the architect to employ that are applicable to a wide range of systems. The book is filled with guidance for applying these concepts and methods.

R.N. Taylor, N. Medvidović, E. Dashofy, risks could result from a small solution space, Software Architecture: Foundations, Theory, and from extremely demanding quality requirePractice [41*]

text of software engineering; the design process; architecture modeling, analysis and visualization; and chapters on several conErder, Pureur and Woods, Continuous cerns including implementation, deployment, Architecture in Practice: Software Architecture in adaptation, non-functional properties, trust the Age of Agility and DevOps. [15] and security.

ments or from possible high-risk failures. The risk-driven approach is harmonious This is a comprehensive textbook on many with low-ceremony and agile approaches. aspects of software architecture, including Architecting, as argued by Fairbanks, is not key ideas; software architecture in the con- just for architects — but is relevant to all developers.

This book shows how "classical" thinking P. Clements et al. Documenting Software about software architecture has evolved in Architecture: Views and Beyond, 2nd edition [9]. the present day in the contexts of agile, cloudbased and DevOps approaches to software development by providing practical guidance on a range of quality and cross-cutting conexamples to express architectures that stake- cerns including security, resilience, scalability

This book provides best practices on capturing software architectures, using guidance and holders can build, use, and maintain that and integration of emerging technologies. system. The book introduces a 3-way categorization of views and therefore viewpoints: into module, component and connector, and alloREFERENCES cation viewtypes, providing numerous examples of each.

Brown, Software Architecture for Developers [5]

- [1] M. Ali Babar, and I. Gorton, "Software Architecture Review: The State of the Practice", IEEE Computer, July 2009.

Brown provides an overview of software [2*] L. Bass, P. Clements, and R. Kazman, architecture topics from the perspective of Software Architecture in Practice, 4th edia developer. He discusses common architection, 2021. ture drivers including architecture principles, quality concerns, constraints and functional [3] L. Bass, J. Ivers, M.H. Klein, and requirements. He has an in-depth discussion P. Merson, Reasoning Frameworks, of the role of the architect in a development CMU/SEI-2005-TR-007, 2005. setting and requisite knowledge and skills for architects. He focuses on the practical issues [4*] F. Brooks, The Design of Design, of architecture in the delivery process and Addison-Wesley, 2010. on managing risk. An appendix provides a case study.

Fairbanks, Just Enough Software Architecture: A risk-driven approach [16]

- [5] S. Brown, Software Architecture for Developers, 2018, http://leanpub.com/ software-architecture-for-developers.
2. [6*] D. Budgen, Software Design: Creating Solutions for Ill-Structured Problems, 3rd Edition, CRC Press, 2021.

Fairbanks offers a risk-driven approach to architecting within the context of development: do just enough software architecture to mitigate the identified risks where those [7] F. Buschmann, R. Meunier, H.

- Rohnert, P. Sommerlad, and M. Stal, Pattern Oriented Software Architecture, John Wiley &amp; Sons, 1996.
- [8] H. Cervantes, R Kazman, Designing Software Architectures: A Practical Approach, 2nd ed., Addison-Wesley, 2024.
- [9] P. Clements et al., Documenting Software Architecture: Views and Beyond, 2nd edition Addison-Wesley, 2011.
- [10] P. Clements, R. Kazman, M. Klein, Evaluating Software Architectures, Addison-Wesley, 2001.
- [11] M.E. Conway, "How Do Committees Invent?" Datamation, 14(4), 28-31, 1968.
- [12] E.W. Dijkstra, "On the role of scientific thought", 1974, available at https://www. cs.utexas.edu/users/EWD/transcriptions/ EWD04xx/EWD447.html.
- [13] T. Earl, SOA Design Patterns, Prentice-Hall, 2009
- [14] P. Eeles, and P. Cripps, The Process of Software Architecting, Addison Wesley, 2010.
- [15]M. Erder, P. Pureur and E. Woods, Continuous Architecture in Practice: Software Architecture in the Age of Agility and DevOps, Addison-Wesley, 2021.
- [16] G. Fairbanks, Just Enough Software Architecture: A Risk-Driven Approach, Marshall &amp; Brainerd, 2010.
- [17] E. Fernandez-Buglioni, Security Patterns in Practice: Designing Secure Architectures Using Software Patterns, Wiley, 2013.
- [18] R.T. Fielding and R.N. Taylor, Principled design of the modern web architecture, ACM Transactions on Internet Technology, 2(2), 115–150, 2002.
- [19] M. Fowler, D. Rice, M. Foemmel, E. Hieatt, R. Mee and R. Stafford, Patterns of Enterprise Application Architecture, Addison-Wesley, 2003.
- [20] C. Hofmeister, P.B. Kruchten, R.L. Nord, H. Obbink, A. Ran, and P. America, "A general model of software architecture design derived from five industrial approaches", The Journal of Systems and Software, 80, 106-126, 2007.
- [21] C. Hofmeister, R.L. Nord, and D. Soni, Applied Software Architecture, AddisonWesley, 2000.
- [22] ISO/IEC/IEEE 24765:2017 Systems and Software Engineering — Vocabulary, 2nd ed. 2017.
- [23]ISO/IEC/IEEE 42010:2011, Systems and software engineering — Architecture description.
- [24] R. Kazman, S. Haziyev, A. Yakuba, and D.A. Tamburri, Managing Energy Consumption as an Architectural Quality Attribute, IEEE Software, 35(5), 102–107, 2018
- [25]P.B. Kruchten, The "4+1" View Model of Architecture, IEEE Software 12(6), 1995.
- [26] P.B. Kruchten, R.L. Nord, and I. Ozkaya, Managing Technical Debt: Reducing Friction in Software Development. Addison-Wesley, 2019.
- [27] Z. Li, P. Liang and P. Avgeriou, Architecture viewpoints for documenting architectural technical debt. Software Quality Assurance, Elsevier, 2016.
- [28] Alan MacCormack, John Rusnak &amp; Carliss Baldwin, Exploring the Duality between Product and Organizational

- Architectures: A Test of the 'Mirroring' Hypothesis. Research Policy, 41:1309-1324,2012
- [29*] M.W. Maier and E. Rechtin, The Art of Systems Architecting, 3rd edition, CRC Press, 2021.
- [30]N. Medvidović, D.S. Rosenblum, D.F. Redmiles and J.E. Robbins, Modeling software architectures in the Unified Modeling Language, ACM Transactions on Software Engineering and Methodology, 11(1), 2–57, 2002
- [31] H. Obbink et al., Report on Software Architecture Review and Assessment (SARA), version 1.0, available at https:// philippe.kruchten.com/architecture/ SARAv1.pdf, 2002.
- [35] E. Poort, H. van Vliet, RCDA: Architecting as a Risk- and Cost Management Discipline, Journal of Systems and Software, https://www .cs.vu.nl/~hans/publications/y2012 /JSS-RCDA.pdf, 2012
- [36] R. Prieto-Diaz and J.M. Neighbors, "Module Interconnection Languages", Journal of Systems and Software, 6(4), 307–334, 1986.
- [37] C. Richardson, Microservices Patterns, Manning Publications, 2019
- [38*] N. Rozanski and E. Woods, Software Systems Architecture: Working with Stakeholders Using Viewpoints and Perspectives, 2nd edition, AddisonWesley, 2011.
- [32] D.L. Parnas, "On the criteria to be used in decomposing systems into modules", Communications of the ACM 15(12), 1053-1058, 1972.
- [33] D.L. Parnas and D.M. Weiss, "Active Design Reviews: Principles and Practices", Proceedings of 8th International Conference on Software Engineering, 215-222, 1985.
- [34] D. Perry, A. Wolf, Foundations for the study of software architecture, ACM SIGSOFT Software Engineering Notes, 17(4), 40–52, 1992
- [39] M. Shaw and D. Garlan, Software Architecture: Perspectives on an Emerging Discipline, Prentice Hall, 1996.
- [40*]I. Sommerville, Software Engineering, 10th edition, 2016.
- [41*] R.N. Taylor, N. Medvidović, E. Dashofy, Software Architecture: Foundations, Theory, and Practice, Wiley, 2009
- [42] R. Weinreich and G. Buchgeher, Towards supporting the software architecture life cycle, The Journal of Systems and Software, 85, 546–561, 2012.